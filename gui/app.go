package gui

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"trustdrop/assets"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
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

	// Advanced/Security features
	securityStatus   *widget.Label
	checkSecurityBtn *widget.Button
	exportLogBtn     *widget.Button
	advancedSection  *widget.Card
	showAdvanced     bool
	toggleAdvanced   *widget.Button

	transferManager *transfer.TransferManager
}

func NewTrustDropApp() (*TrustDropApp, error) {
	myApp := app.New()

	// Set the icon using the fixed icon data
	myApp.SetIcon(assets.GetAppIcon())

	window := myApp.NewWindow("TrustDrop - Secure File Transfer")
	window.Resize(fyne.NewSize(450, 400))
	window.CenterOnScreen()

	transferManager, err := transfer.NewTransferManager()
	if err != nil {
		return nil, fmt.Errorf("failed to create transfer manager: %w", err)
	}

	app := &TrustDropApp{
		app:             myApp,
		window:          window,
		transferManager: transferManager,
		showAdvanced:    false, // Hidden by default
	}

	// Set up callbacks with better error handling
	transferManager.SetStatusCallback(app.onStatusUpdate)
	transferManager.SetProgressCallback(app.onProgressUpdate)

	return app, nil
}

func (t *TrustDropApp) Run() {
	t.setupUI()

	// Show the local peer ID in console for debugging
	fmt.Printf("TrustDrop started. Your Peer ID: %s\n", t.transferManager.GetLocalPeerID())

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
	t.progressBar.Hide()

	// Current file label
	t.currentFileLabel = widget.NewLabel("")
	t.currentFileLabel.Hide()

	// Remaining files label
	t.remainingLabel = widget.NewLabel("")
	t.remainingLabel.Hide()

	// Cancel button
	t.cancelBtn = widget.NewButton("Cancel Transfer", func() {
		t.transferManager.CancelTransfer()
		t.hideProgress()
		t.enableControls()
		t.statusLabel.SetText("Status: Transfer cancelled")
	})
	t.cancelBtn.Importance = widget.DangerImportance
	t.cancelBtn.Hide()

	// Advanced toggle button (subtle, at bottom)
	t.toggleAdvanced = widget.NewButtonWithIcon("Security", theme.SettingsIcon(), t.toggleAdvancedSection)
	t.toggleAdvanced.Importance = widget.LowImportance

	// Security status with friendly language
	t.securityStatus = widget.NewLabelWithStyle("✅ All transfers are secured and logged",
		fyne.TextAlignCenter, fyne.TextStyle{Italic: true})

	// User-friendly button names
	t.checkSecurityBtn = widget.NewButton("Check Security", t.onCheckSecurity)
	t.checkSecurityBtn.Importance = widget.LowImportance

	t.exportLogBtn = widget.NewButton("Export Security Log", t.onExportSecurityLog)
	t.exportLogBtn.Importance = widget.LowImportance

	// Create main sections
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
			widget.NewLabel("Select files to send:"),
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

	// Advanced security section (hidden by default)
	t.advancedSection = widget.NewCard("Security Details", "",
		container.NewVBox(
			t.securityStatus,
			widget.NewSeparator(),
			container.NewHBox(
				t.checkSecurityBtn,
				t.exportLogBtn,
			),
		),
	)
	t.advancedSection.Hide() // Hidden by default

	// Main layout
	mainContent := container.NewVBox(
		yourIDSection,
		widget.NewSeparator(),
		receiveSection,
		widget.NewSeparator(),
		sendSection,
		widget.NewSeparator(),
		progressSection,
		t.advancedSection, // Hidden by default
	)

	// Footer with subtle security toggle
	footer := container.NewBorder(
		widget.NewSeparator(),
		nil,
		nil,
		t.toggleAdvanced,
		nil,
	)

	// Combine everything
	content := container.NewBorder(
		nil,
		footer,
		nil,
		nil,
		container.NewScroll(mainContent),
	)

	t.window.SetContent(container.NewPadded(content))
}

func (t *TrustDropApp) onStartReceive() {
	peerID := strings.TrimSpace(t.peerIDEntry.Text)
	if peerID == "" {
		dialog.ShowError(fmt.Errorf("please enter the sender's code"), t.window)
		return
	}

	fmt.Printf("Starting receive with peer ID: %s\n", peerID)

	// Start receiving
	t.disableControls()
	t.statusLabel.SetText("Status: Connecting to sender...")

	go func() {
		err := t.transferManager.StartReceive(peerID)
		if err != nil {
			fmt.Printf("Receive error: %v\n", err)
			t.window.Canvas().Refresh(t.statusLabel)
			dialog.ShowError(fmt.Errorf("failed to receive: %v", err), t.window)
			t.enableControls()
			t.statusLabel.SetText(fmt.Sprintf("Status: Failed - %v", err))
		}
	}()
}

func (t *TrustDropApp) onSelectFiles() {
	if t.transferManager.IsTransferActive() {
		dialog.ShowError(fmt.Errorf("transfer already in progress"), t.window)
		return
	}

	// Create a custom dialog to allow selecting files or folders
	selectDialog := dialog.NewCustom("Select Files or Folder", "Cancel", container.NewVBox(
		widget.NewLabel("Choose what to send:"),
		widget.NewButton("Select Files", func() {
			t.selectMultipleFiles()
		}),
		widget.NewButton("Select Folder", func() {
			t.selectFolder()
		}),
	), t.window)
	selectDialog.Show()
}

func (t *TrustDropApp) selectMultipleFiles() {
	// Create file open dialog
	fileDialog := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
		if err != nil {
			dialog.ShowError(fmt.Errorf("failed to open file: %v", err), t.window)
			return
		}
		if reader == nil {
			return
		}
		reader.Close()

		path := reader.URI().Path()

		// Handle URI schemes on different platforms
		if runtime.GOOS == "windows" && strings.HasPrefix(path, "/") {
			// Remove leading slash on Windows paths
			path = path[1:]
		}

		fmt.Printf("Selected file for sending: %s\n", path)

		// Note: Fyne doesn't support multiple file selection natively
		// For now, we'll send single files, but you could implement a custom solution
		t.startSend([]string{path})

	}, t.window)

	// Set starting directory
	homeDir, _ := os.UserHomeDir()
	listableURI := storage.NewFileURI(homeDir)
	if lister, ok := listableURI.(fyne.ListableURI); ok {
		fileDialog.SetLocation(lister)
	}

	fileDialog.Show()
}

func (t *TrustDropApp) selectFolder() {
	dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
		if err != nil {
			if strings.Contains(err.Error(), "operation not permitted") {
				dialog.ShowError(fmt.Errorf("permission denied: cannot access this folder. Please select a folder you have access to, or grant permission in System Preferences > Security & Privacy"), t.window)
			} else {
				dialog.ShowError(fmt.Errorf("failed to open folder: %v", err), t.window)
			}
			return
		}
		if uri == nil {
			return
		}

		path := uri.Path()

		// Handle URI schemes on different platforms
		if runtime.GOOS == "windows" && strings.HasPrefix(path, "/") {
			// Remove leading slash on Windows paths
			path = path[1:]
		}

		// Verify we can access the path
		if _, err := os.Stat(path); err != nil {
			dialog.ShowError(fmt.Errorf("cannot access folder: %v", err), t.window)
			return
		}

		fmt.Printf("Selected path for sending: %s\n", path)
		t.startSend([]string{path})
	}, t.window)
}

func (t *TrustDropApp) startSend(paths []string) {
	if len(paths) == 0 {
		dialog.ShowError(fmt.Errorf("no files selected"), t.window)
		return
	}

	// Verify all paths exist
	for _, path := range paths {
		if _, err := os.Stat(path); err != nil {
			dialog.ShowError(fmt.Errorf("cannot access path %s: %v", path, err), t.window)
			return
		}
	}

	t.disableControls()
	t.showProgress()

	t.currentFileLabel.SetText("Preparing transfer...")
	t.remainingLabel.SetText("Encrypting files...")
	t.progressBar.SetValue(0)
	t.statusLabel.SetText("Status: Preparing to send...")

	// Show the peer ID that receiver needs to use
	dialog.ShowInformation("Ready to Send",
		fmt.Sprintf("Your Peer ID: %s\n\nThe receiver must enter this code to receive the files.\n\nFiles will be encrypted with AES-256 before transfer.",
			t.transferManager.GetLocalPeerID()),
		t.window)

	// Start the transfer
	go func() {
		err := t.transferManager.SendFiles(paths)
		if err != nil {
			fmt.Printf("Send error: %v\n", err)
			fyne.Do(func() {
				dialog.ShowError(fmt.Errorf("failed to send: %v", err), t.window)
				t.enableControls()
				t.hideProgress()
				t.statusLabel.SetText(fmt.Sprintf("Status: Failed - %v", err))
			})
		}
	}()
}

func (t *TrustDropApp) onStatusUpdate(status string) {
	fmt.Printf("Status update: %s\n", status)

	// All GUI updates must be wrapped in fyne.Do() to run on main thread
	fyne.Do(func() {
		// Update status label
		t.statusLabel.SetText("Status: " + status)

		// Show progress during active transfers
		if strings.Contains(status, "Waiting") || strings.Contains(status, "Preparing") ||
			strings.Contains(status, "Connected") || strings.Contains(status, "Sending") ||
			strings.Contains(status, "Receiving") || strings.Contains(status, "Encrypting") ||
			strings.Contains(status, "Decrypting") {
			t.showProgress()
		}

		// Hide progress and re-enable controls when transfer is complete or failed
		if strings.Contains(status, "successfully") || strings.Contains(status, "cancelled") ||
			strings.Contains(status, "failed") || strings.Contains(status, "Failed") {
			t.hideProgress()
			t.enableControls()

			// Update security status if advanced section is visible
			if t.showAdvanced {
				t.updateSecurityStatus()
			}

			// Show completion dialog for successful transfers
			if strings.Contains(status, "successfully") {
				message := "Your files have been transferred successfully and securely!"
				if strings.Contains(status, "decrypted") {
					message = "Files received and decrypted successfully!\n\nThey are saved in: data/received/"
				}
				dialog.ShowInformation("Transfer Complete", message, t.window)
			}
		}
	})
}

func (t *TrustDropApp) onProgressUpdate(progress transfer.TransferProgress) {
	// All GUI updates must be wrapped in fyne.Do() to run on main thread
	fyne.Do(func() {
		// Update progress bar and labels
		t.progressBar.SetValue(progress.PercentComplete / 100.0)
		t.currentFileLabel.SetText("Current File: " + progress.CurrentFile)
		t.remainingLabel.SetText(fmt.Sprintf("Files Remaining: %d", progress.FilesRemaining))
	})
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
	t.progressBar.SetValue(0)
}

func (t *TrustDropApp) disableControls() {
	t.connectBtn.Disable()
	t.selectBtn.Disable()
	t.peerIDEntry.Disable()
	if t.showAdvanced {
		t.checkSecurityBtn.Disable()
		t.exportLogBtn.Disable()
	}
}

func (t *TrustDropApp) enableControls() {
	t.connectBtn.Enable()
	t.selectBtn.Enable()
	t.peerIDEntry.Enable()
	if t.showAdvanced {
		t.checkSecurityBtn.Enable()
		t.exportLogBtn.Enable()
	}
}

func (t *TrustDropApp) toggleAdvancedSection() {
	t.showAdvanced = !t.showAdvanced
	if t.showAdvanced {
		t.advancedSection.Show()
		t.toggleAdvanced.SetText("Hide Security")
		t.updateSecurityStatus()
		// Resize window to accommodate
		t.window.Resize(fyne.NewSize(450, 500))
	} else {
		t.advancedSection.Hide()
		t.toggleAdvanced.SetText("Security")
		// Resize back to normal
		t.window.Resize(fyne.NewSize(450, 400))
	}
}

func (t *TrustDropApp) updateSecurityStatus() {
	logger := t.transferManager.GetLogger()
	info := logger.GetBlockchainInfo()

	if info["ledger_healthy"].(bool) {
		transfers := info["chain_length"].(int) - 1 // Subtract genesis block
		if transfers < 0 {
			transfers = 0
		}
		t.securityStatus.SetText(fmt.Sprintf("✅ Security log is intact (%d transfers recorded)", transfers))
	} else {
		t.securityStatus.SetText("⚠️ Security log needs attention")
	}
}

func (t *TrustDropApp) onCheckSecurity() {
	logger := t.transferManager.GetLogger()
	valid, err := logger.VerifyLedger()

	if err != nil {
		dialog.ShowError(fmt.Errorf("security check failed: %v", err), t.window)
		return
	}

	if valid {
		info := logger.GetBlockchainInfo()
		transfers := info["chain_length"].(int) - 1
		dialog.ShowInformation("Security Check Passed",
			fmt.Sprintf("✅ Your transfer history is secure and intact.\n\n%d transfers have been safely recorded.", transfers),
			t.window)
	} else {
		dialog.ShowError(fmt.Errorf("security log may have been tampered with"), t.window)
	}

	t.updateSecurityStatus()
}

func (t *TrustDropApp) onExportSecurityLog() {
	dialog.ShowFileSave(func(writer fyne.URIWriteCloser, err error) {
		if err != nil {
			dialog.ShowError(err, t.window)
			return
		}
		if writer == nil {
			return
		}
		defer writer.Close()

		logger := t.transferManager.GetLogger()
		filename := writer.URI().Path()

		if err := logger.ExportLedger(filename); err != nil {
			dialog.ShowError(fmt.Errorf("failed to export security log: %v", err), t.window)
			return
		}

		dialog.ShowInformation("Export Complete",
			"Your security log has been exported successfully!",
			t.window)
	}, t.window)
}
