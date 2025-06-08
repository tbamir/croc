package gui

import (
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"trustdrop/transfer"
)

type TrustDropApp struct {
	app    fyne.App
	window fyne.Window

	// Main UI elements
	mainContent     *fyne.Container
	sendCard        *widget.Card
	receiveCard     *widget.Card
	progressCard    *widget.Card
	successCard     *widget.Card

	// Send elements
	peerIDDisplay   *widget.Label
	copyButton      *widget.Button
	selectButton    *widget.Button
	waitingLabel    *widget.Label

	// Receive elements
	codeEntry       *widget.Entry
	receiveButton   *widget.Button

	// Progress elements
	progressBar     *widget.ProgressBar
	statusLabel     *widget.Label
	detailLabel     *widget.Label
	cancelButton    *widget.Button

	// Success elements
	successMessage  *widget.Label
	doneButton      *widget.Button

	// State
	transferManager *transfer.TransferManager
	currentView     string
	mutex           sync.Mutex
}

func NewTrustDropApp() (*TrustDropApp, error) {
	myApp := app.New()
	myApp.Settings().SetTheme(&trustDropTheme{})

	window := myApp.NewWindow("TrustDrop")
	window.Resize(fyne.NewSize(420, 500))
	window.CenterOnScreen()

	transferManager, err := transfer.NewTransferManager()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize: %w", err)
	}

	app := &TrustDropApp{
		app:             myApp,
		window:          window,
		transferManager: transferManager,
		currentView:     "main",
	}

	// Set up callbacks with proper thread safety
	transferManager.SetStatusCallback(app.onStatusUpdate)
	transferManager.SetProgressCallback(app.onProgressUpdate)

	return app, nil
}

func (t *TrustDropApp) Run() {
	t.setupUI()
	t.window.ShowAndRun()
}

func (t *TrustDropApp) setupUI() {
	// Create all UI cards
	t.createMainView()
	t.createSendView()
	t.createReceiveView()
	t.createProgressView()
	t.createSuccessView()

	// Start with main view
	t.showMainView()
}

func (t *TrustDropApp) createMainView() {
	// App logo/title
	title := widget.NewLabelWithStyle("TrustDrop", 
		fyne.TextAlignCenter, 
		fyne.TextStyle{Bold: true})
	title.TextStyle.Bold = true
	
	subtitle := widget.NewLabel("Secure File Transfer")
	subtitle.Alignment = fyne.TextAlignCenter

	// Send button - large and prominent
	sendBtn := widget.NewButton("Send Files", func() {
		t.showSendView()
	})
	sendBtn.Importance = widget.HighImportance

	// Receive button - large and prominent  
	receiveBtn := widget.NewButton("Receive Files", func() {
		t.showReceiveView()
	})
	receiveBtn.Importance = widget.MediumImportance

	// Simple explanation
	explanation := widget.NewLabel("Send files securely to anyone,\nanywhere, without the cloud")
	explanation.Alignment = fyne.TextAlignCenter
	explanation.Wrapping = fyne.TextWrapWord

	// Layout
	content := container.NewVBox(
		container.NewPadded(container.NewVBox(
			title,
			subtitle,
		)),
		widget.NewSeparator(),
		container.NewPadded(container.NewVBox(
			explanation,
			layout.NewSpacer(),
			container.NewGridWithColumns(1,
				container.NewPadded(sendBtn),
				container.NewPadded(receiveBtn),
			),
		)),
	)

	t.mainContent = container.NewCenter(content)
}

func (t *TrustDropApp) createSendView() {
	// Your code display
	t.peerIDDisplay = widget.NewLabelWithStyle(
		t.transferManager.GetLocalPeerID(),
		fyne.TextAlignCenter,
		fyne.TextStyle{Monospace: true, Bold: true})

	t.copyButton = widget.NewButtonWithIcon("Copy Code", theme.ContentCopyIcon(), func() {
		t.window.Clipboard().SetContent(t.transferManager.GetLocalPeerID())
		t.copyButton.SetText("Copied!")
		time.AfterFunc(2*time.Second, func() {
			t.copyButton.SetText("Copy Code")
		})
	})

	// Select files button
	t.selectButton = widget.NewButton("Choose Files to Send", t.onSelectFiles)
	t.selectButton.Importance = widget.HighImportance
	t.selectButton.Icon = theme.FolderOpenIcon()

	// Waiting indicator
	t.waitingLabel = widget.NewLabel("Share the code above with the receiver")
	t.waitingLabel.Alignment = fyne.TextAlignCenter
	t.waitingLabel.Hide()

	// Back button
	backBtn := widget.NewButtonWithIcon("Back", theme.NavigateBackIcon(), func() {
		if t.transferManager.IsTransferActive() {
			dialog.ShowConfirm("Cancel Transfer?", 
				"Are you sure you want to cancel the current transfer?",
				func(cancel bool) {
					if cancel {
						t.transferManager.CancelTransfer()
						t.showMainView()
					}
				}, t.window)
		} else {
			t.showMainView()
		}
	})

	// Layout
	codeCard := widget.NewCard("", "Your Transfer Code:",
		container.NewVBox(
			container.NewPadded(t.peerIDDisplay),
			t.copyButton,
		))

	content := container.NewVBox(
		container.NewBorder(nil, nil, backBtn, nil),
		codeCard,
		widget.NewSeparator(),
		container.NewPadded(container.NewVBox(
			widget.NewLabel("1. Click below to select files"),
			t.selectButton,
			widget.NewLabel("2. Share your code with the receiver"),
			t.waitingLabel,
		)),
	)

	t.sendCard = widget.NewCard("Send Files", "", content)
}

func (t *TrustDropApp) createReceiveView() {
	// Code entry
	t.codeEntry = widget.NewEntry()
	t.codeEntry.SetPlaceHolder("Enter sender's code")

	// Receive button
	t.receiveButton = widget.NewButton("Start Receiving", func() {
		code := strings.TrimSpace(t.codeEntry.Text)
		if code == "" {
			dialog.ShowError(fmt.Errorf("Please enter the sender's code"), t.window)
			return
		}
		t.onStartReceive(code)
	})
	t.receiveButton.Importance = widget.HighImportance
	t.receiveButton.Icon = theme.DownloadIcon()

	// Back button
	backBtn := widget.NewButtonWithIcon("Back", theme.NavigateBackIcon(), func() {
		t.showMainView()
	})

	// Instructions
	instructions := widget.NewLabel("Enter the code from the sender\nto receive files")
	instructions.Alignment = fyne.TextAlignCenter
	instructions.Wrapping = fyne.TextWrapWord

	// Layout
	content := container.NewVBox(
		container.NewBorder(nil, nil, backBtn, nil),
		instructions,
		widget.NewSeparator(),
		container.NewPadded(container.NewVBox(
			t.codeEntry,
			t.receiveButton,
		)),
	)

	t.receiveCard = widget.NewCard("Receive Files", "", content)
}

func (t *TrustDropApp) createProgressView() {
	// Progress bar
	t.progressBar = widget.NewProgressBar()

	// Status labels
	t.statusLabel = widget.NewLabelWithStyle("Connecting...", 
		fyne.TextAlignCenter, 
		fyne.TextStyle{Bold: true})
	
	t.detailLabel = widget.NewLabel("")
	t.detailLabel.Alignment = fyne.TextAlignCenter
	t.detailLabel.Wrapping = fyne.TextWrapWord

	// Cancel button
	t.cancelButton = widget.NewButton("Cancel", func() {
		dialog.ShowConfirm("Cancel Transfer?",
			"Are you sure you want to cancel this transfer?",
			func(cancel bool) {
				if cancel {
					t.transferManager.CancelTransfer()
					t.showMainView()
				}
			}, t.window)
	})
	t.cancelButton.Importance = widget.DangerImportance

	// Animation placeholder (you could add a spinner here)
	progressAnimation := canvas.NewText("◐", theme.PrimaryColor())
	progressAnimation.TextSize = 48
	progressAnimation.Alignment = fyne.TextAlignCenter

	// Layout
	content := container.NewVBox(
		container.NewPadded(container.NewCenter(progressAnimation)),
		widget.NewSeparator(),
		container.NewPadded(container.NewVBox(
			t.statusLabel,
			t.progressBar,
			t.detailLabel,
			layout.NewSpacer(),
			container.NewCenter(t.cancelButton),
		)),
	)

	t.progressCard = widget.NewCard("Transfer in Progress", "", content)
}

func (t *TrustDropApp) createSuccessView() {
	// Success icon (checkmark)
	successIcon := canvas.NewText("✓", theme.SuccessColor())
	successIcon.TextSize = 72
	successIcon.Alignment = fyne.TextAlignCenter

	// Success message
	t.successMessage = widget.NewLabelWithStyle("Transfer Complete!", 
		fyne.TextAlignCenter,
		fyne.TextStyle{Bold: true})

	// Location label
	locationLabel := widget.NewLabel("Files saved to:\ndata/received/")
	locationLabel.Alignment = fyne.TextAlignCenter

	// Done button
	t.doneButton = widget.NewButton("Done", func() {
		t.showMainView()
	})
	t.doneButton.Importance = widget.HighImportance

	// Layout
	content := container.NewVBox(
		container.NewPadded(container.NewCenter(successIcon)),
		widget.NewSeparator(),
		container.NewPadded(container.NewVBox(
			t.successMessage,
			locationLabel,
			layout.NewSpacer(),
			t.doneButton,
		)),
	)

	t.successCard = widget.NewCard("Success", "", content)
}

// View switching methods
func (t *TrustDropApp) showMainView() {
	t.mutex.Lock()
	t.currentView = "main"
	t.mutex.Unlock()
	
	t.window.SetContent(container.NewPadded(t.mainContent))
	t.codeEntry.SetText("") // Clear any entered code
}

func (t *TrustDropApp) showSendView() {
	t.mutex.Lock()
	t.currentView = "send"
	t.mutex.Unlock()
	
	t.window.SetContent(container.NewPadded(t.sendCard))
	t.waitingLabel.Hide()
}

func (t *TrustDropApp) showReceiveView() {
	t.mutex.Lock()
	t.currentView = "receive"
	t.mutex.Unlock()
	
	t.window.SetContent(container.NewPadded(t.receiveCard))
}

func (t *TrustDropApp) showProgressView() {
	t.mutex.Lock()
	t.currentView = "progress"
	t.mutex.Unlock()
	
	t.progressBar.SetValue(0)
	t.window.SetContent(container.NewPadded(t.progressCard))
}

func (t *TrustDropApp) showSuccessView(message string) {
	t.mutex.Lock()
	t.currentView = "success"
	t.mutex.Unlock()
	
	t.successMessage.SetText(message)
	t.window.SetContent(container.NewPadded(t.successCard))
}

// File selection
func (t *TrustDropApp) onSelectFiles() {
	dialog.NewCustom("Select what to send", "Cancel", 
		container.NewVBox(
			widget.NewButton("Select Files", func() {
				t.selectMultipleFiles()
			}),
			widget.NewButton("Select Folder", func() {
				t.selectFolder()
			}),
		), t.window).Show()
}

func (t *TrustDropApp) selectMultipleFiles() {
	fileDialog := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
		if err != nil || reader == nil {
			return
		}
		reader.Close()

		path := reader.URI().Path()
		if runtime.GOOS == "windows" && strings.HasPrefix(path, "/") {
			path = path[1:]
		}

		t.startSend([]string{path})
	}, t.window)

	homeDir, _ := os.UserHomeDir()
	listableURI := storage.NewFileURI(homeDir)
	if lister, ok := listableURI.(fyne.ListableURI); ok {
		fileDialog.SetLocation(lister)
	}

	fileDialog.Show()
}

func (t *TrustDropApp) selectFolder() {
	dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
		if err != nil || uri == nil {
			if err != nil && strings.Contains(err.Error(), "operation not permitted") {
				dialog.ShowError(fmt.Errorf("Permission denied. Please grant folder access in System Preferences"), t.window)
			}
			return
		}

		path := uri.Path()
		if runtime.GOOS == "windows" && strings.HasPrefix(path, "/") {
			path = path[1:]
		}

		// Verify access
		if _, err := os.Stat(path); err != nil {
			dialog.ShowError(fmt.Errorf("Cannot access this folder"), t.window)
			return
		}

		t.startSend([]string{path})
	}, t.window)
}

func (t *TrustDropApp) startSend(paths []string) {
	if len(paths) == 0 {
		return
	}

	// Update UI to show waiting state
	t.selectButton.Disable()
	t.waitingLabel.Show()
	t.waitingLabel.SetText("Waiting for receiver to connect...")

	// Start transfer in background
	go func() {
		err := t.transferManager.SendFiles(paths)
		if err != nil {
			fyne.CurrentApp().SendNotification(&fyne.Notification{
				Title:   "Transfer Failed",
				Content: err.Error(),
			})
			t.window.Canvas().Content().Refresh()
		}
	}()
}

func (t *TrustDropApp) onStartReceive(code string) {
	t.showProgressView()
	t.statusLabel.SetText("Connecting to sender...")

	go func() {
		err := t.transferManager.StartReceive(code)
		if err != nil {
			fyne.CurrentApp().SendNotification(&fyne.Notification{
				Title:   "Transfer Failed", 
				Content: err.Error(),
			})
			// Return to receive view on error
			time.Sleep(time.Second) // Brief delay so user sees the error
			t.showReceiveView()
		}
	}()
}

// Callbacks from transfer manager
func (t *TrustDropApp) onStatusUpdate(status string) {
	t.mutex.Lock()
	currentView := t.currentView
	t.mutex.Unlock()

	// Simplify status messages for users
	userStatus := t.simplifyStatus(status)

	// Update UI based on current view
	switch currentView {
	case "send":
		if strings.Contains(status, "Connected") {
			t.showProgressView()
			t.statusLabel.SetText("Sending files...")
		}
	case "receive":
		if strings.Contains(status, "Waiting") || strings.Contains(status, "Connecting") {
			// Already in progress view from onStartReceive
		}
	case "progress":
		t.statusLabel.SetText(userStatus)
	}

	// Handle completion
	if strings.Contains(status, "successfully") {
		var message string
		if strings.Contains(status, "decrypted") {
			message = "Files received successfully!\n\nSaved to: data/received/"
		} else {
			message = "Files sent successfully!"
		}
		t.showSuccessView(message)
	}

	// Handle cancellation
	if strings.Contains(status, "cancelled") {
		t.showMainView()
	}
}

func (t *TrustDropApp) onProgressUpdate(progress transfer.TransferProgress) {
	t.mutex.Lock()
	currentView := t.currentView
	t.mutex.Unlock()

	if currentView == "progress" {
		// Update progress bar
		t.progressBar.SetValue(progress.PercentComplete / 100.0)

		// Update detail label with current file
		if progress.CurrentFile != "" {
			fileName := filepath.Base(progress.CurrentFile)
			if progress.FilesRemaining > 0 {
				t.detailLabel.SetText(fmt.Sprintf("%s\n(%d files remaining)", 
					fileName, progress.FilesRemaining))
			} else {
				t.detailLabel.SetText(fileName)
			}
		}

		// Update status for different stages
		if progress.PercentComplete < 10 {
			t.statusLabel.SetText("Starting transfer...")
		} else if progress.PercentComplete < 90 {
			t.statusLabel.SetText("Transferring files...")
		} else {
			t.statusLabel.SetText("Finishing up...")
		}
	}
}

func (t *TrustDropApp) simplifyStatus(status string) string {
	// Convert technical status messages to user-friendly ones
	replacements := map[string]string{
		"Waiting for sender":        "Connecting to sender...",
		"Preparing files":           "Getting ready...",
		"Encrypting":                "Securing your files...",
		"Decrypting":                "Receiving files...",
		"Connected":                 "Connected! Starting transfer...",
		"Sending":                   "Sending files...",
		"Receiving":                 "Receiving files...",
		"successfully":              "All done!",
		"failed":                    "Something went wrong",
		"cancelled":                 "Transfer cancelled",
		"Connecting to sender":      "Looking for sender...",
		"Decrypting and restoring": "Organizing received files...",
	}

	simplified := status
	for tech, simple := range replacements {
		if strings.Contains(strings.ToLower(status), strings.ToLower(tech)) {
			simplified = simple
			break
		}
	}

	return simplified
}

// Custom theme for a modern, clean look
type trustDropTheme struct{}

func (t trustDropTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNamePrimary:
		return color.NRGBA{R: 0x00, G: 0x7A, B: 0xFF, A: 0xFF} // Bright blue
	case theme.ColorNameButton:
		return color.NRGBA{R: 0x00, G: 0x7A, B: 0xFF, A: 0xFF}
	case theme.ColorNameForeground:
		if variant == theme.VariantLight {
			return color.NRGBA{R: 0x21, G: 0x21, B: 0x21, A: 0xFF}
		}
		return color.NRGBA{R: 0xF0, G: 0xF0, B: 0xF0, A: 0xFF}
	case theme.ColorNameBackground:
		if variant == theme.VariantLight {
			return color.NRGBA{R: 0xFA, G: 0xFA, B: 0xFA, A: 0xFF}
		}
		return color.NRGBA{R: 0x1A, G: 0x1A, B: 0x1A, A: 0xFF}
	}
	return theme.DefaultTheme().Color(name, variant)
}

func (t trustDropTheme) Font(style fyne.TextStyle) fyne.Resource {
	return theme.DefaultTheme().Font(style)
}

func (t trustDropTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

func (t trustDropTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNamePadding:
		return 8
	case theme.SizeNameInlineIcon:
		return 24
	case theme.SizeNameText:
		return 15
	}
	return theme.DefaultTheme().Size(name)
}