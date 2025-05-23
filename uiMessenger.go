package main

import (
	"context"
	"fmt"
	"image/color"
	"log"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

func getMessageBody(ev *event.Event) string {
	if body, ok := ev.Content.Raw["body"].(string); ok {
		return body
	}
	return ""
}

func makeBubble(text string, right bool) fyne.CanvasObject {
	txt := canvas.NewText(text, theme.TextColor())
	if right {
		txt.Alignment = fyne.TextAlignTrailing
	} else {
		txt.Alignment = fyne.TextAlignLeading
	}
	txt.TextSize = theme.TextSize()

	var bgColor color.Color
	if right {
		bgColor = color.NRGBA{R: 0xAD, G: 0xD8, B: 0xE6, A: 0xFF}
	} else {
		bgColor = color.NRGBA{R: 0xCC, G: 0xCC, B: 0xCC, A: 0xFF}
	}
	bg := canvas.NewRectangle(bgColor)
	bg.CornerRadius = 8

	padded := container.NewPadded(txt)
	bubble := container.NewMax(bg, padded)

	if right {
		return container.NewHBox(layout.NewSpacer(), bubble)
	}
	return container.NewHBox(bubble, layout.NewSpacer())
}

func makeDateSeparator(date string) fyne.CanvasObject {
	txt := canvas.NewText(date, theme.TextColor())
	txt.TextSize = 12
	rect := canvas.NewRectangle(theme.ButtonColor())
	rect.CornerRadius = 10
	rect.SetMinSize(fyne.NewSize(80, 20))
	chip := container.NewMax(rect, container.NewCenter(txt))
	return container.NewHBox(layout.NewSpacer(), chip, layout.NewSpacer())
}

func startMessenger(myApp fyne.App) {
	bgImage := canvas.NewImageFromFile("assets/background.png")
	bgImage.FillMode = canvas.ImageFillStretch

	icon, err := fyne.LoadResourceFromPath("assets/icon.ico")
	if err != nil {
		fmt.Println("Error:", err)
	} else {
		myApp.SetIcon(icon)
	}

	ctx := context.Background()
	cfg := loadConfig("config.json")
	client := mustLogin(cfg)
	_ = client.SetPresence(ctx, mautrix.ReqPresence{Presence: "online"})

	var (
		mu           sync.Mutex
		history      = container.NewVBox()
		chatScroller = container.NewVScroll(history)
		selectedRoom id.RoomID
		prevToken    string
	)
	chatScroller = container.NewVScroll(history)
	chatScroller.SetMinSize(fyne.NewSize(0, 500))

	syncer := client.Syncer.(*mautrix.DefaultSyncer)
	syncer.OnEventType(event.EventMessage, func(ctx context.Context, ev *event.Event) {
		body := getMessageBody(ev)
		log.Printf("LIVE: %s, %s: %s", ev.RoomID, ev.Sender, body)
		if ev.Sender == client.UserID {
			return
		}
		mu.Lock()
		defer mu.Unlock()
		if ev.RoomID != selectedRoom {
			return
		}
		fyne.Do(func() {
			history.Add(makeBubble(body, ev.Sender == client.UserID))
			history.Refresh()
			chatScroller.ScrollToBottom()
			myApp.SetIcon(icon)
		})
	})

	go func() {
		for {
			if err := client.Sync(); err != nil {
				log.Println("Sync error:", err)
			}
			time.Sleep(time.Second)
		}
	}()

	win := myApp.NewWindow("QalqanDS")
	win.Resize(fyne.NewSize(600, 500))
	win.CenterOnScreen()

	jr, err := client.JoinedRooms(ctx)
	if err != nil {
		dialog.ShowError(err, win)
		return
	}
	type Room struct {
		ID   id.RoomID
		Name string
	}
	rooms := make([]Room, len(jr.JoinedRooms))
	for i, rid := range jr.JoinedRooms {
		var rn event.RoomNameEventContent
		_ = client.StateEvent(ctx, rid, event.StateRoomName, "", &rn)
		name := string(rid)
		if rn.Name != "" {
			name = rn.Name
		}
		rooms[i] = Room{ID: rid, Name: name}
	}
	roomsList := widget.NewList(
		func() int { return len(rooms) },
		func() fyne.CanvasObject {
			t := canvas.NewText("", color.White)
			t.Alignment = fyne.TextAlignLeading
			return t
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			t := o.(*canvas.Text)
			t.Text = rooms[i].Name
			t.Color = color.White
			t.Refresh()
		},
	)
	roomsList.Refresh()

	iconSwitchCall, err := fyne.LoadResourceFromPath("assets/call.png")
	if err != nil {
		fmt.Println("Error loading icon:", err)
		iconSwitchCall = theme.CancelIcon()
	}
	switchBtn := widget.NewButtonWithIcon("", iconSwitchCall, func() {
		switchCall(myApp)
	})

	iconInterfaceMenu, err := fyne.LoadResourceFromPath("assets/interfaceMenu.png")
	if err != nil {
		fmt.Println("Error loading icon:", err)
		iconInterfaceMenu = theme.CancelIcon()
	}
	interfaceMenuBtn := widget.NewButtonWithIcon("", iconInterfaceMenu, func() {
		// interfaceMenu(myApp)
		dialog.ShowInformation(
			"Interface settings",
			"Interface settings will be here",
			win,
		)
	})

	topBar := container.NewHBox(layout.NewSpacer(), switchBtn, interfaceMenuBtn)

	msgEntry := widget.NewEntry()
	msgEntry.SetPlaceHolder("Type message...")
	sendBtn := widget.NewButtonWithIcon("", theme.MailSendIcon(), nil)
	inputRow := container.NewBorder(nil, nil, nil, sendBtn, msgEntry)

	rightPane := container.NewBorder(nil, inputRow, nil, nil, chatScroller)
	split := container.NewHSplit(roomsList, rightPane)
	split.SetOffset(0.25)
	mainContent := container.NewBorder(topBar, nil, nil, nil, split)
	win.SetContent(container.NewMax(bgImage, mainContent))
	win.Show()

	roomsList.OnSelected = func(i widget.ListItemID) {
		mu.Lock()
		selectedRoom = rooms[i].ID
		mu.Unlock()

		history.Objects = nil
		history.Refresh()

		resp, err := client.Messages(ctx, selectedRoom, "", "", mautrix.DirectionBackward, nil, 50)
		if err != nil {
			dialog.ShowError(err, win)
			return
		}
		prevToken = resp.Start

		log.Printf("История: %d событий", len(resp.Chunk))
		for i, ev := range resp.Chunk {
			log.Printf("Event #%d: Type=%s, Sender=%s, Raw=%+v", i, ev.Type, ev.Sender, ev.Content)
		}

		history.Add(widget.NewButton("Load more", func() {
			mu.Lock()
			more, err := client.Messages(ctx, selectedRoom, prevToken, "", mautrix.DirectionBackward, nil, 50)
			mu.Unlock()
			if err != nil || len(more.Chunk) == 0 {
				return
			}
			prevToken = more.Start
			mu.Lock()
			for idx := len(more.Chunk) - 1; idx >= 0; idx-- {
				ev := more.Chunk[idx]
				if ev.Type == event.EventMessage {
					body := getMessageBody(ev)
					history.Objects = append(
						[]fyne.CanvasObject{makeBubble(body, ev.Sender == client.UserID)},
						history.Objects...,
					)
				}
			}
			history.Refresh()
			chatScroller.ScrollToTop()
			mu.Unlock()
		}))

		history.Add(makeDateSeparator(time.Now().Format("Jan 2, 2006")))

		for idx := len(resp.Chunk) - 1; idx >= 0; idx-- {
			ev := resp.Chunk[idx]
			if ev.Type == event.EventMessage {
				body := getMessageBody(ev)
				history.Add(makeBubble(body, ev.Sender == client.UserID))
			}
		}
		history.Refresh()
		chatScroller.Refresh()

		fyne.Do(func() {
			chatScroller.ScrollToBottom()
		})

		sendBtn.OnTapped = func() {
			text := msgEntry.Text
			if text == "" {
				return
			}
			if _, err := client.SendText(ctx, selectedRoom, text); err != nil {
				dialog.ShowError(err, win)
				return
			}
			mu.Lock()
			history.Add(makeBubble(text, true))
			mu.Unlock()
			history.Refresh()
			msgEntry.SetText("")
			chatScroller.ScrollToBottom()
		}
	}

	if len(rooms) > 0 {
		roomsList.Select(0)
	}
}
