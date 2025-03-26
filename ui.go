package main

import (
	"QalqanDS/qalqan"
	"bytes"
	"encoding/hex"
	"fmt"
	"io"
	"math/rand"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func animateResize(w fyne.Window, newSize fyne.Size) {
	oldSize := w.Canvas().Size()

	stepCount := 10
	delay := 20 * time.Millisecond

	widthStep := (newSize.Width - oldSize.Width) / float32(stepCount)
	heightStep := (newSize.Height - oldSize.Height) / float32(stepCount)

	go func() {
		for i := 0; i < stepCount; i++ {
			time.Sleep(delay)
			w.Resize(fyne.NewSize(
				oldSize.Width+widthStep*float32(i),
				oldSize.Height+heightStep*float32(i),
			))
		}
		w.Resize(newSize)
	}()
}

func useAndDeleteSessionKey() []uint8 {
	if len(session_keys) == 0 || len(session_keys[0]) == 0 {
		fmt.Println("No session keys available")
		return nil
	}

	key := session_keys[0][0][:qalqan.DEFAULT_KEY_LEN]
	rkey := make([]uint8, qalqan.EXPKLEN)
	qalqan.Kexp(key, qalqan.DEFAULT_KEY_LEN, qalqan.BLOCKLEN, rkey)
	for i := 0; i < qalqan.DEFAULT_KEY_LEN; i++ {
		session_keys[0][0][i] = 0
	}
	copy(session_keys[0][:], session_keys[0][1:])
	session_keys[0][len(session_keys[0])-1] = [qalqan.DEFAULT_KEY_LEN]byte{}
	if session_keys[0][0] == [32]byte{} {
		session_keys = session_keys[1:]
	}
	return rkey
}

func useAndDeleteCircleKey(randomNum int) []uint8 {
	if len(circle_keys) == 0 || len(circle_keys[0]) == 0 {
		fmt.Println("No session keys available")
		return nil
	}
	key := circle_keys[randomNum][:qalqan.DEFAULT_KEY_LEN]
	rkey := make([]uint8, qalqan.EXPKLEN)
	qalqan.Kexp(key, qalqan.DEFAULT_KEY_LEN, qalqan.BLOCKLEN, rkey)
	return rkey
}

var session_keys [][100][qalqan.DEFAULT_KEY_LEN]byte
var circle_keys [10][qalqan.DEFAULT_KEY_LEN]byte
var sessionKeyCount int = 100

func InitUI(w fyne.Window) {

	bgImage := canvas.NewImageFromFile("assets/background.png")
	bgImage.FillMode = canvas.ImageFillStretch

	icon, err := fyne.LoadResourceFromPath("assets/icon.ico")
	if err != nil {
		fmt.Println("Error:", err)
	} else {
		w.SetIcon(icon)
	}

	logs := widget.NewMultiLineEntry()
	logs.SetPlaceHolder("Logs output...")
	logs.Disable()
	logs.Wrapping = fyne.TextWrapWord
	logs.Scroll = container.ScrollBoth

	logs.Resize(fyne.NewSize(400, 150))

	rKey := make([]uint8, qalqan.EXPKLEN)

	selectSource := widget.NewSelect([]string{"File", "Key"}, nil)
	selectSource.PlaceHolder = "Select source of key"

	passwordEntry := widget.NewPasswordEntry()
	passwordEntry.SetPlaceHolder("Enter a password...")

	hashLabel := widget.NewLabelWithStyle(
		"Hash of key",
		fyne.TextAlignCenter,
		fyne.TextStyle{Bold: true},
	)

	hashValue := widget.NewEntry()
	hashValue.Disable()

	hashContainer := container.NewVBox(
		layout.NewSpacer(),
		hashLabel,
		layout.NewSpacer(),
		container.NewCenter(container.NewGridWrap(fyne.NewSize(515, 40), hashValue)),
		layout.NewSpacer(),
	)

	sessionKeys := widget.NewRadioGroup([]string{"Session keys"}, nil)

	keysLeftEntry := widget.NewEntry()
	keysLeftEntry.SetPlaceHolder("Keys left")
	keysLeftEntry.Disable()
	smallKeysLeftEntry := container.NewCenter(container.NewGridWrap(fyne.NewSize(170, 40), keysLeftEntry))

	leftContainer := container.NewVBox(
		container.NewCenter(sessionKeys),
		smallKeysLeftEntry,
	)

	okButton := widget.NewButton("OK", func() {
		password := passwordEntry.Text
		if password == "" {
			dialog.ShowInformation("Error", "Enter a password!", w)
			return
		}
		sessionKeyCount = 100
		keysLeftEntry.SetText(fmt.Sprintf("%d", sessionKeyCount))

		logs.SetText("Password entered: " + password)
		fileDialog := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil {
				logs.SetText("Error opening file: " + err.Error())
				return
			}
			if reader == nil {
				logs.SetText("No file selected.")
				return
			}
			defer reader.Close()

			data, err := io.ReadAll(reader)
			if err != nil {
				logs.SetText("Failed to read file: " + err.Error())
				return
			}
			ostream := bytes.NewBuffer(data)
			kikey := make([]byte, qalqan.DEFAULT_KEY_LEN)
			ostream.Read(kikey[:qalqan.DEFAULT_KEY_LEN])
			key := qalqan.Hash512(password)
			keyBytes := hex.EncodeToString(key[:])
			hashValue.SetText(keyBytes)
			qalqan.Kexp(key[:], qalqan.DEFAULT_KEY_LEN, qalqan.BLOCKLEN, rKey)
			for i := 0; i < qalqan.DEFAULT_KEY_LEN; i += qalqan.BLOCKLEN {
				qalqan.DecryptOFB(kikey[i:i+qalqan.BLOCKLEN], rKey, qalqan.DEFAULT_KEY_LEN, qalqan.BLOCKLEN, kikey[i:i+qalqan.BLOCKLEN])
			}
			if len(data) < qalqan.BLOCKLEN {
				logs.SetText("The file is too short")
				return
			}
			imitstream := bytes.NewBuffer(data)
			imitFile := make([]byte, qalqan.BLOCKLEN)
			rimitkey := make([]byte, qalqan.EXPKLEN)
			qalqan.Kexp(kikey, qalqan.DEFAULT_KEY_LEN, qalqan.BLOCKLEN, rimitkey)
			qalqan.Qalqan_Imit(uint64(len(data)-qalqan.BLOCKLEN), rimitkey, imitstream, imitFile)
			rimit := make([]byte, qalqan.BLOCKLEN)
			imitstream.Read(rimit[:qalqan.BLOCKLEN])
			if !bytes.Equal(rimit, imitFile) {
				logs.SetText("The file is corrupted")
			}
			circle_keys = [10][qalqan.DEFAULT_KEY_LEN]byte{}
			qalqan.LoadCircleKeys(data, ostream, rKey, &circle_keys)
			qalqan.LoadSessionKeys(data, ostream, rKey, &session_keys)
			fmt.Println("Session keys loaded successfully")
			dialog.ShowInformation("Success", "Keys loaded successfully!", w)

			defer func() {
				if r := recover(); r != nil {
					logs.SetText(fmt.Sprintf("File open failed: %v", r))
				}
			}()
		}, w)

		fileDialog.SetFilter(storage.NewExtensionFileFilter([]string{".bin"}))
		fileDialog.Show()
	})

	iconClear, err := fyne.LoadResourceFromPath("assets/clear.png")
	if err != nil {
		fmt.Println("Ошибка загрузки иконки:", err)
		iconClear = theme.CancelIcon()
	}

	clearLogsButton := container.NewGridWrap(fyne.NewSize(120, 40),
		widget.NewButtonWithIcon(
			"Clear log",
			iconClear,
			func() {
				logs.SetText("")
				fmt.Println("Логи очищены")
			},
		),
	)
	centeredButton := container.NewCenter(clearLogsButton)

	logsContainer := container.NewVBox(
		logs,
		centeredButton,
	)

	fromEntry := widget.NewEntry()
	fromEntry.SetPlaceHolder("From")
	fromEntry.Hide()

	toEntry := widget.NewEntry()
	toEntry.SetPlaceHolder("To")
	toEntry.Hide()

	dateEntry := widget.NewEntry()
	dateEntry.SetPlaceHolder("Date")
	dateEntry.Hide()

	regEntry := widget.NewEntry()
	regEntry.SetPlaceHolder("Registration No.")
	regEntry.Hide()

	tableBar := container.NewGridWithColumns(4,
		fromEntry,
		toEntry,
		dateEntry,
		regEntry,
	)

	outputLabel := widget.NewMultiLineEntry()
	outputLabel.SetMinRowsVisible(6)
	outputLabel.Disable()

	updateOutput := func() {
		outputLabel.SetText(
			"From: " + fromEntry.Text + "\n" +
				"To: " + toEntry.Text + "\n" +
				"Date: " + dateEntry.Text + "\n" +
				"Registration No.: " + regEntry.Text,
		)
	}

	fromEntry.OnChanged = func(string) { updateOutput() }
	toEntry.OnChanged = func(string) { updateOutput() }
	dateEntry.OnChanged = func(string) { updateOutput() }
	regEntry.OnChanged = func(string) { updateOutput() }

	updateOutput()

	messageSend := widget.NewMultiLineEntry()
	messageSend.SetPlaceHolder("Your message...")
	messageSend.Enable()
	messageSend.Wrapping = fyne.TextWrapWord
	messageSend.Scroll = container.ScrollBoth
	messageSend.Resize(fyne.NewSize(500, 150))
	messageSend.Hide()

	iconEncrMessage, err := fyne.LoadResourceFromPath("assets/encryptMessage.png")
	if err != nil {
		fmt.Println("Ошибка загрузки иконки:", err)
		iconEncrMessage = theme.CancelIcon()
	}

	createdMessageButton := widget.NewButtonWithIcon(
		"Encrypt a message",
		iconEncrMessage,
		func() {
			messageSend.SetText("")
			fmt.Println("Очищено")

			dialog.ShowInformation("Success", "Message encrypted successfully!", w)
		},
	)

	createdMessageButton.Hide()
	centeredButtonMessage := container.NewCenter(createdMessageButton)

	messageContainer := container.NewVBox(
		messageSend,
		centeredButtonMessage,
	)

	customMessage := widget.NewRadioGroup([]string{"Custom message"}, func(selected string) {
		isEnabled := selected == "Custom message"

		if isEnabled {
			fromEntry.Show()
			toEntry.Show()
			dateEntry.Show()
			regEntry.Show()
			messageSend.Show()
			createdMessageButton.Show()
			animateResize(w, fyne.NewSize(675, 650))
		} else {
			fromEntry.Hide()
			toEntry.Hide()
			dateEntry.Hide()
			regEntry.Hide()
			messageSend.Hide()
			createdMessageButton.Hide()
			animateResize(w, fyne.NewSize(600, 300))
		}
	})

	selectModeEntry := widget.NewSelect(
		[]string{"OFB", "ECB"},
		func(selected string) {
			fmt.Println("Выбран режим:", selected)
		})
	selectModeEntry.PlaceHolder = "Select mode"
	selectModeEntry.Disable()

	modeExperts := widget.NewRadioGroup([]string{"Mode (for experts)"}, func(selected string) {
		if selected == "Mode (for experts)" {
			selectModeEntry.Enable()
		} else {
			selectModeEntry.Disable()
		}
	})
	modeExperts.SetSelected("")

	smallSelectModeEntry := container.NewCenter(container.NewGridWrap(fyne.NewSize(170, 40), selectModeEntry))

	rightContainer := container.NewVBox(
		container.NewCenter(modeExperts),
		smallSelectModeEntry,
	)

	topRow := container.NewHBox(
		layout.NewSpacer(),
		container.NewGridWrap(fyne.NewSize(170, 40), selectSource),
		layout.NewSpacer(),
		container.NewGridWrap(fyne.NewSize(180, 40), passwordEntry),
		layout.NewSpacer(),
		container.NewGridWrap(fyne.NewSize(65, 40), okButton),
		layout.NewSpacer(),
	)

	keyTypeSelect := widget.NewSelect(
		[]string{"Circular", "Session"},
		func(selected string) {
			fmt.Println("Выбран тип ключа:", selected)
		},
	)

	keyTypeSelect.PlaceHolder = "Select key type"

	centerContainer := container.NewVBox(
		container.NewCenter(customMessage),
		container.NewCenter(container.NewGridWrap(fyne.NewSize(170, 40), keyTypeSelect)),
	)

	sessionModeContainer := container.NewHBox(
		layout.NewSpacer(),
		leftContainer,
		layout.NewSpacer(),
		centerContainer,
		layout.NewSpacer(),
		rightContainer,
		layout.NewSpacer(),
	)

	iconEncrypt, err := fyne.LoadResourceFromPath("assets/encrypt.png")
	if err != nil {
		fmt.Println("Ошибка загрузки иконки:", err)
		iconEncrypt = theme.ConfirmIcon()
	}

	encryptButton := widget.NewButtonWithIcon(
		"Encrypt a file",
		iconEncrypt,
		func() {
			if len(session_keys) == 0 || len(session_keys[0]) == 0 {
				dialog.ShowError(fmt.Errorf("please load the encryption keys first"), w)
				return
			}
			fileDialog := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
				if err != nil {
					logs.SetText("Error opening file: " + err.Error())
					return
				}
				if reader == nil {
					logs.SetText("No file selected.")
					return
				}
				defer reader.Close()

				data, err := io.ReadAll(reader)
				if err != nil {
					logs.SetText("Failed to read file: " + err.Error())
					return
				}

				ostream := bytes.NewBuffer(data)
				sstream := &bytes.Buffer{}

				defer func() {
					if r := recover(); r != nil {
						logs.SetText(fmt.Sprintf("Encryption failed: %v", r))
					}
				}()

				iv := make([]byte, qalqan.BLOCKLEN)
				for i := range qalqan.BLOCKLEN {
					iv[i] = byte(rand.Intn(256))
				}

				/*
					rKey := useAndDeleteSessionKey() // test use of encryption on session keys
					if rKey == nil {
						logOutput.SetText("No session key available for encryption.")
						return
					}
				*/
				fmt.Println("circle_keys:", circle_keys)
				randomNum := 8 //rand.Intn(10)
				fmt.Println("Key's number:", randomNum)
				rKey := useAndDeleteCircleKey(randomNum)
				if rKey == nil {
					logs.SetText("No session key available for encryption.")
					return
				}

				qalqan.EncryptOFB_File(len(data), rKey, iv, ostream, sstream)

				saveDialog := dialog.NewFileSave(func(writer fyne.URIWriteCloser, err error) {
					if err != nil {
						logs.SetText("Error saving file: " + err.Error())
						return
					}
					if writer == nil {
						logs.SetText("No file selected for saving.")
						return
					}
					defer writer.Close()

					_, err = writer.Write(sstream.Bytes())
					if err != nil {
						logs.SetText("Failed to save encrypted file: " + err.Error())
						return
					}
					if sessionKeyCount > 0 {
						sessionKeyCount--
						keysLeftEntry.SetText(fmt.Sprintf("%d", sessionKeyCount))
					}
					logs.SetText("File successfully encrypted and saved!")
				}, w)

				saveDialog.SetFileName("encrypted_file.qln")
				saveDialog.Show()
			}, w)
			fileDialog.Show()
		})

	iconDecrypt, err := fyne.LoadResourceFromPath("assets/decrypt.png")
	if err != nil {
		fmt.Println("Ошибка загрузки иконки:", err)
		iconDecrypt = theme.CancelIcon()
	}

	decryptButton := widget.NewButtonWithIcon(
		"Decrypt a file",
		iconDecrypt,
		func() {
			if len(session_keys) == 0 || len(session_keys[0]) == 0 {
				dialog.ShowError(fmt.Errorf("please load the encryption keys first"), w)
				return
			}
			fileDialog := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
				if err != nil {
					logs.SetText("Error opening file: " + err.Error())
					return
				}
				if reader == nil {
					logs.SetText("No file selected.")
					return
				}
				defer reader.Close()

				data, err := io.ReadAll(reader)
				if err != nil {
					logs.SetText("Failed to read file: " + err.Error())
					return
				}

				if len(data) < qalqan.BLOCKLEN {
					logs.SetText("Invalid file: too small to contain IV.")
					return
				}

				iv := data[:qalqan.BLOCKLEN]
				encryptedData := data[qalqan.BLOCKLEN:]
				/*
					rKey := useAndDeleteSessionKey() // test use of decryption on session keys
					if rKey == nil {
						logOutput.SetText("No session key available for decryption.")
						return
					}
				*/

				rKey := useAndDeleteCircleKey(8) // test use of decryption on session keys
				if rKey == nil {
					logs.SetText("No session key available for decryption.")
					return
				}
				ostream := bytes.NewBuffer(encryptedData)
				sstream := &bytes.Buffer{}

				defer func() {
					if r := recover(); r != nil {
						logs.SetText(fmt.Sprintf("Decryption failed: %v", r))
					}
				}()

				qalqan.DecryptOFB_File(int(len(encryptedData)), rKey, iv, ostream, sstream)

				saveDialog := dialog.NewFileSave(func(writer fyne.URIWriteCloser, err error) {
					if err != nil {
						logs.SetText("Error saving file: " + err.Error())
						return
					}
					if writer == nil {
						logs.SetText("No file selected for saving.")
						return
					}
					defer writer.Close()

					_, err = writer.Write(sstream.Bytes())
					if err != nil {
						logs.SetText("Failed to save decrypted file: " + err.Error())
						return
					}

					logs.SetText("File successfully decrypted and saved!")
				}, w)

				saveDialog.SetFileName("decrypted_file.txt")
				saveDialog.Show()
			}, w)

			fileDialog.SetFilter(storage.NewExtensionFileFilter([]string{".qln"}))
			fileDialog.Show()
		},
	)

	buttonContainer := container.NewHBox(
		layout.NewSpacer(),
		encryptButton,
		layout.NewSpacer(),
		decryptButton,
		layout.NewSpacer(),
	)

	mainUI := container.NewVBox(
		widget.NewLabel(" "),
		topRow,
		widget.NewLabel(" "),
		hashContainer,
		widget.NewLabel(" "),
		sessionModeContainer,
		widget.NewLabel(" "),
		buttonContainer,
		widget.NewLabel(" "),
		logsContainer,
		widget.NewLabel(" "),
		tableBar,
		messageContainer,
	)

	content := container.NewStack(bgImage, mainUI)

	w.SetContent(content)
}
