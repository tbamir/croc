package gui

import (
	"fmt"
	"os"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"trustdrop/transfer"
)

type TrustDropApp struct {
	app              fyne.App
	window           fyne.Window
	localPeerIDLabel *widget.Label
	copyButton       *widget.Button
	peerIDEntry      *widget.Entry
	statusLabel      *widget.Label
	connectBtn       *widget.Button
	selectBtn        *widget.Button
	progressBar      *widget.ProgressBar
	currentFileLabel *widget.Label
	remainingLabel   *widget.Label
	cancelBtn        *widget.Button

	transferManager *transfer.TransferManager
}

func NewTrustDropApp() *TrustDropApp {
	myApp := app.New()
	myApp.SetIcon(fyne.NewStaticResource("icon", []byte{})) // TODO: Add actual icon

	window := myApp.NewWindow("TrustDrop")
	window.Resize(fyne.NewSize(450, 380))
	window.CenterOnScreen()

	transferManager := transfer.NewTransferManager()

	app := &TrustDropApp{
		app:             myApp,
		window:          window,
		transferManager: transferManager,
	}

	// Set up callbacks
	transferManager.SetStatusCallback(app.onStatusUpdate)
	transferManager.SetProgressCallback(app.onProgressUpdate)

	return app
}

func (t *TrustDropApp) Run() {
	t.setupUI()
	t.window.ShowAndRun()
}

func (t *TrustDropApp) setupUI() {
	// Local Peer ID display
	localPeerID := t.transferManager.GetLocalPeerID()
	t.localPeerIDLabel = widget.NewLabel(localPeerID)
	t.localPeerIDLabel.TextStyle = fyne.TextStyle{Monospace: true}

	// Copy button for local peer ID
	t.copyButton = widget.NewButton("Copy", func() {
		t.window.Clipboard().SetContent(localPeerID)
		dialog.ShowInformation("Copied", "Peer ID copied to clipboard!", t.window)
	})
	t.copyButton.Importance = widget.MediumImportance

	// Peer ID input for connecting to others
	t.peerIDEntry = widget.NewEntry()
	t.peerIDEntry.SetPlaceHolder("Enter code from sender")

	// Connect button - now starts receiving
	t.connectBtn = widget.NewButton("Start Receiving", t.onStartReceive)
	t.connectBtn.Importance = widget.HighImportance

	// Status label
	t.statusLabel = widget.NewLabel("Status: Ready")

	// File selection button
	t.selectBtn = widget.NewButton("Send Files/Folders", t.onSelectFiles)

	// Progress bar
	t.progressBar = widget.NewProgressBar()
	t.progressBar.Hide() // Hide initially

	// Current file label
	t.currentFileLabel = widget.NewLabel("")
	t.currentFileLabel.Hide() // Hide initially

	// Remaining files label
	t.remainingLabel = widget.NewLabel("")
	t.remainingLabel.Hide() // Hide initially

	// Cancel button
	t.cancelBtn = widget.NewButton("Cancel Transfer", func() {
		t.transferManager.CancelTransfer()
		t.hideProgress()
		t.enableControls()
	})
	t.cancelBtn.Importance = widget.DangerImportance
	t.cancelBtn.Hide()

	// Create sections with improved layout
	yourIDSection := widget.NewCard("Your Peer ID", "Share this code with others to send you files:",
		container.NewVBox(
			container.NewBorder(nil, nil, nil, t.copyButton, t.localPeerIDLabel),
		),
	)

	receiveSection := widget.NewCard("Receive Files", "",
		container.NewVBox(
			widget.NewLabel("Enter sender's code:"),
			t.peerIDEntry,
			container.NewPadded(t.connectBtn),
		),
	)

	sendSection := widget.NewCard("Send Files", "",
		container.NewVBox(
			widget.NewLabel("Send files using your Peer ID above:"),
			container.NewPadded(t.selectBtn),
		),
	)

	progressSection := widget.NewCard("Transfer Status", "",
		container.NewVBox(
			t.statusLabel,
			widget.NewSeparator(),
			widget.NewLabel("Progress:"),
			t.progressBar,
			t.currentFileLabel,
			t.remainingLabel,
			t.cancelBtn,
		),
	)

	// Main layout with improved spacing
	content := container.NewVBox(
		yourIDSection,
		widget.NewSeparator(),
		receiveSection,
		widget.NewSeparator(),
		sendSection,
		widget.NewSeparator(),
		progressSection,
	)

	scrollableContent := container.NewScroll(content)
	t.window.SetContent(container.NewPadded(scrollableContent))
}

func (t *TrustDropApp) onStartReceive() {
	peerID := t.peerIDEntry.Text
	if peerID == "" {
		dialog.ShowError(fmt.Errorf("please enter the sender's code"), t.window)
		return
	}

	// Start receiving
	t.disableControls()
	go func() {
		err := t.transferManager.StartReceive(peerID)
		if err != nil {
			dialog.ShowError(err, t.window)
			t.enableControls()
		}
	}()
}

func (t *TrustDropApp) onSelectFiles() {
	if t.transferManager.IsTransferActive() {
		dialog.ShowError(fmt.Errorf("transfer already in progress"), t.window)
		return
	}

	dialog.ShowFolderOpen(func(folder fyne.ListableURI, err error) {
		if err != nil {
			if strings.Contains(err.Error(), "operation not permitted") {
				dialog.ShowError(fmt.Errorf("permission denied: cannot access this folder. Please select a folder you have access to, or grant permission in System Preferences > Security & Privacy"), t.window)
			} else {
				dialog.ShowError(fmt.Errorf("failed to open folder: %v", err), t.window)
			}
			return
		}
		if folder == nil {
			return
		}

		folderPath := folder.Path()
		if folderPath == "" {
			dialog.ShowError(fmt.Errorf("invalid folder path"), t.window)
			return
		}

		if _, err := os.Stat(folderPath); err != nil {
			dialog.ShowError(fmt.Errorf("cannot access folder: %v", err), t.window)
			return
		}

		t.startSend(folderPath)
	}, t.window)
}

func (t *TrustDropApp) startSend(path string) {
	if path == "" {
		dialog.ShowError(fmt.Errorf("invalid file path"), t.window)
		return
	}

	if _, err := os.Stat(path); err != nil {
		dialog.ShowError(fmt.Errorf("cannot access path %s: %v", path, err), t.window)
		return
	}

	t.disableControls()
	t.showProgress()

	t.currentFileLabel.SetText("Preparing transfer...")
	t.remainingLabel.SetText("Analyzing files...")
	t.progressBar.SetValue(0)

	// Start the transfer
	go func() {
		err := t.transferManager.SendFiles([]string{path})
		if err != nil {
			dialog.ShowError(err, t.window)
			t.enableControls()
			t.hideProgress()
		}
	}()
}

func (t *TrustDropApp) onStatusUpdate(status string) {
	t.statusLabel.SetText("Status: " + status)

	// Show progress during active transfers
	if status == "Waiting for sender..." || status == "Connecting to receiver..." ||
		status == "Connected! Receiving files..." || status == "Connected! Sending files..." {
		t.showProgress()
	}

	// Hide progress and re-enable controls when transfer is complete
	if status == "Files received successfully!" || status == "Files sent successfully!" ||
		status == "Transfer cancelled" ||
		(status != "Ready" && (status[:6] == "Failed" || status[:7] == "Receive" && status[len(status)-6:] == "failed" || status[:4] == "Send" && status[len(status)-6:] == "failed")) {
		t.hideProgress()
		t.enableControls()
	}
}

func (t *TrustDropApp) onProgressUpdate(progress transfer.TransferProgress) {
	t.progressBar.SetValue(progress.PercentComplete / 100.0)
	t.currentFileLabel.SetText("Current File: " + progress.CurrentFile)
	t.remainingLabel.SetText(fmt.Sprintf("Files Remaining: %d", progress.FilesRemaining))
}

func (t *TrustDropApp) showProgress() {
	t.progressBar.Show()
	t.currentFileLabel.Show()
	t.remainingLabel.Show()
	t.cancelBtn.Show()
}

func (t *TrustDropApp) hideProgress() {
	t.progressBar.Hide()
	t.currentFileLabel.Hide()
	t.remainingLabel.Hide()
	t.cancelBtn.Hide()
}

func (t *TrustDropApp) disableControls() {
	t.connectBtn.Disable()
	t.selectBtn.Disable()
	t.peerIDEntry.Disable()
}

func (t *TrustDropApp) enableControls() {
	t.connectBtn.Enable()
	t.selectBtn.Enable()
	t.peerIDEntry.Enable()
}
