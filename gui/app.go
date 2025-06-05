package gui

import (
	"fmt"

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
	receiveBtn       *widget.Button

	transferManager *transfer.TransferManager
	isConnected     bool
}

func NewTrustDropApp() *TrustDropApp {
	myApp := app.New()
	myApp.SetIcon(fyne.NewStaticResource("icon", []byte{})) // TODO: Add actual icon

	window := myApp.NewWindow("TrustDrop")
	window.Resize(fyne.NewSize(450, 350))
	window.CenterOnScreen()

	transferManager := transfer.NewTransferManager()

	app := &TrustDropApp{
		app:             myApp,
		window:          window,
		transferManager: transferManager,
		isConnected:     false,
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
	t.peerIDEntry.SetPlaceHolder("Enter code from other peer")

	// Connect button
	t.connectBtn = widget.NewButton("Connect", t.onConnect)
	t.connectBtn.Importance = widget.HighImportance

	// Status label
	t.statusLabel = widget.NewLabel("Status: Not Connected")

	// File selection button
	t.selectBtn = widget.NewButton("Select Files/Folders to Send", t.onSelectFiles)
	t.selectBtn.Disable() // Disable until connected

	// Receive button
	receiveBtn := widget.NewButton("Wait for Files", func() {
		if !t.isConnected {
			dialog.ShowError(fmt.Errorf("please connect to a peer first"), t.window)
			return
		}

		// Show progress UI
		t.progressBar.Show()
		t.currentFileLabel.Show()
		t.remainingLabel.Show()

		t.currentFileLabel.SetText("Waiting for files...")
		t.remainingLabel.SetText("Status: Ready to receive")
		t.progressBar.SetValue(0)

		// Start receiving
		go func() {
			err := t.transferManager.ReceiveFiles()
			if err != nil {
				dialog.ShowError(err, t.window)
			}
		}()
	})
	receiveBtn.Disable() // Disable until connected

	// Progress bar
	t.progressBar = widget.NewProgressBar()
	t.progressBar.Hide() // Hide initially

	// Current file label
	t.currentFileLabel = widget.NewLabel("")
	t.currentFileLabel.Hide() // Hide initially

	// Remaining files label
	t.remainingLabel = widget.NewLabel("")
	t.remainingLabel.Hide() // Hide initially

	// Create sections with improved layout
	yourIDSection := widget.NewCard("Your Peer ID", "Share this code with the other peer:",
		container.NewVBox(
			container.NewBorder(nil, nil, nil, t.copyButton, t.localPeerIDLabel),
		),
	)

	connectionSection := widget.NewCard("Connect to Peer", "",
		container.NewVBox(
			widget.NewLabel("Enter code from other peer:"),
			t.peerIDEntry,
			container.NewPadded(t.connectBtn),
			widget.NewSeparator(),
			t.statusLabel,
		),
	)

	transferSection := widget.NewCard("File Transfer", "",
		container.NewVBox(
			container.NewGridWithColumns(2,
				t.selectBtn,
				receiveBtn,
			),
			widget.NewSeparator(),
			widget.NewLabel("Transfer Progress:"),
			t.progressBar,
			t.currentFileLabel,
			t.remainingLabel,
		),
	)

	// Main layout with improved spacing
	content := container.NewVBox(
		yourIDSection,
		widget.NewSeparator(),
		connectionSection,
		widget.NewSeparator(),
		transferSection,
	)

	scrollableContent := container.NewScroll(content)
	t.window.SetContent(container.NewPadded(scrollableContent))

	// Store receiveBtn reference for enable/disable
	t.receiveBtn = receiveBtn
}

func (t *TrustDropApp) onConnect() {
	peerID := t.peerIDEntry.Text
	if peerID == "" {
		dialog.ShowError(fmt.Errorf("please enter a peer ID"), t.window)
		return
	}

	if t.isConnected {
		// Disconnect
		t.transferManager.Disconnect()
		t.isConnected = false
		t.connectBtn.SetText("Connect")
		t.connectBtn.Importance = widget.HighImportance
		t.selectBtn.Disable()
		t.receiveBtn.Disable()
		return
	}

	// Connect
	t.connectBtn.Disable()
	go func() {
		err := t.transferManager.ConnectToPeer(peerID)
		if err != nil {
			dialog.ShowError(err, t.window)
			t.connectBtn.Enable()
		}
	}()
}

func (t *TrustDropApp) onSelectFiles() {
	// File dialog
	dialog.ShowFolderOpen(func(folder fyne.ListableURI, err error) {
		if err != nil {
			dialog.ShowError(err, t.window)
			return
		}
		if folder == nil {
			return // User cancelled
		}

		// Start file transfer
		t.startTransfer(folder.Path())
	}, t.window)
}

func (t *TrustDropApp) startTransfer(path string) {
	// Show progress UI
	t.progressBar.Show()
	t.currentFileLabel.Show()
	t.remainingLabel.Show()

	t.currentFileLabel.SetText("Preparing transfer...")
	t.remainingLabel.SetText("Files Remaining: Calculating...")
	t.progressBar.SetValue(0)

	// Start the transfer
	go func() {
		err := t.transferManager.SendFiles([]string{path})
		if err != nil {
			dialog.ShowError(err, t.window)
		}
	}()
}

func (t *TrustDropApp) onStatusUpdate(status string) {
	t.statusLabel.SetText("Status: " + status)

	if status == fmt.Sprintf("Connected to peer: %s", t.transferManager.GetConnectedPeerID()) {
		t.isConnected = true
		t.connectBtn.Enable()
		t.connectBtn.SetText("Disconnect")
		t.connectBtn.Importance = widget.DangerImportance
		t.selectBtn.Enable()
		t.receiveBtn.Enable()
	} else if status == "Not Connected" {
		t.isConnected = false
		t.connectBtn.Enable()
		t.connectBtn.SetText("Connect")
		t.connectBtn.Importance = widget.HighImportance
		t.selectBtn.Disable()
		t.receiveBtn.Disable()
	} else if status == "Connecting..." {
		t.connectBtn.SetText("Connecting...")
	}
}

func (t *TrustDropApp) onProgressUpdate(progress transfer.TransferProgress) {
	t.progressBar.SetValue(progress.PercentComplete / 100.0)
	t.currentFileLabel.SetText("Current File: " + progress.CurrentFile)
	t.remainingLabel.SetText(fmt.Sprintf("Files Remaining: %d", progress.FilesRemaining))
}
