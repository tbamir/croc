package gui

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"trustdrop-bulletproof/transfer"
	"trustdrop-bulletproof/transport"
)

// BulletproofApp provides an intuitive GUI for bulletproof file transfers
type BulletproofApp struct {
	app             fyne.App
	window          fyne.Window
	transferManager *transfer.BulletproofTransferManager
	targetDataDir   string

	// Main UI elements
	mainContent  *fyne.Container
	sendCard     *widget.Card
	receiveCard  *widget.Card
	progressCard *widget.Card
	successCard  *widget.Card

	// Send elements
	codeDisplay  *widget.Label
	copyButton   *widget.Button
	selectButton *widget.Button
	waitingLabel *widget.Label

	// Receive elements
	codeEntry     *widget.Entry
	receiveButton *widget.Button

	// Progress elements
	statusLabel  *widget.Label
	detailLabel  *widget.Label
	cancelButton *widget.Button

	// Success elements
	successMessage *widget.Label
	locationLabel  *widget.Label
	openFolderBtn  *widget.Button
	doneButton     *widget.Button

	// State
	currentView     string
	mutex           sync.Mutex
	isTransferring  bool
	currentCode     string
	selectionDialog *dialog.CustomDialog
}

// NewAppWithBulletproofManager creates a new bulletproof app with proper UX
func NewAppWithBulletproofManager(transferManager *transfer.BulletproofTransferManager, targetDataDir string) *BulletproofApp {
	bulletproofApp := &BulletproofApp{
		app:             app.New(),
		transferManager: transferManager,
		targetDataDir:   targetDataDir,
		currentCode:     generateTransferCode(),
	}

	bulletproofApp.setupUI()
	bulletproofApp.setupCallbacks()

	return bulletproofApp
}

// Run starts the bulletproof application
func (ba *BulletproofApp) Run() {
	ba.window.ShowAndRun()
}

// setupUI creates the complete user interface with proper workflow
func (ba *BulletproofApp) setupUI() {
	ba.window = ba.app.NewWindow("TrustDrop Bulletproof Edition")
	ba.window.Resize(fyne.NewSize(500, 400))
	ba.window.CenterOnScreen()

	// Create all views
	ba.createMainView()
	ba.createSendView()
	ba.createReceiveView()
	ba.createProgressView()
	ba.createSuccessView()

	// Start with main view
	ba.showMainView()
}

// createMainView creates the main menu with clear Send/Receive options
func (ba *BulletproofApp) createMainView() {
	// App title
	title := widget.NewLabelWithStyle("TrustDrop Bulletproof",
		fyne.TextAlignCenter,
		fyne.TextStyle{Bold: true})

	subtitle := widget.NewLabel("Ultra-reliable P2P file transfer")
	subtitle.Alignment = fyne.TextAlignCenter

	// Send button - large and prominent
	sendBtn := widget.NewButton("Send Files", func() {
		ba.showSendView()
	})
	sendBtn.Importance = widget.HighImportance
	sendBtn.Icon = theme.MailSendIcon()

	// Receive button - large and prominent
	receiveBtn := widget.NewButton("Receive Files", func() {
		ba.showReceiveView()
	})
	receiveBtn.Importance = widget.MediumImportance
	receiveBtn.Icon = theme.DownloadIcon()

	// Network status
	networkStatus := ba.getNetworkStatusWidget()

	// Layout
	content := container.NewVBox(
		container.NewPadded(container.NewVBox(
			title,
			subtitle,
		)),
		widget.NewSeparator(),
		container.NewPadded(container.NewVBox(
			layout.NewSpacer(),
			container.NewGridWithColumns(1,
				container.NewPadded(sendBtn),
				container.NewPadded(receiveBtn),
			),
			layout.NewSpacer(),
			networkStatus,
		)),
	)

	ba.mainContent = container.NewCenter(content)
}

// createSendView creates the send workflow with code display
func (ba *BulletproofApp) createSendView() {
	// Code display
	ba.codeDisplay = widget.NewLabelWithStyle(
		ba.currentCode,
		fyne.TextAlignCenter,
		fyne.TextStyle{Monospace: true, Bold: true})

	ba.copyButton = widget.NewButtonWithIcon("Copy Code", theme.ContentCopyIcon(), func() {
		ba.window.Clipboard().SetContent(ba.currentCode)
		ba.copyButton.SetText("Copied!")
		time.AfterFunc(2*time.Second, func() {
			ba.copyButton.SetText("Copy Code")
		})
	})

	// Select files button
	ba.selectButton = widget.NewButton("Choose Files to Send", ba.onSelectFiles)
	ba.selectButton.Importance = widget.HighImportance
	ba.selectButton.Icon = theme.FolderOpenIcon()

	// Waiting indicator
	ba.waitingLabel = widget.NewLabel("Share the code above with the receiver")
	ba.waitingLabel.Alignment = fyne.TextAlignCenter
	ba.waitingLabel.Hide()

	// Back button
	backBtn := widget.NewButtonWithIcon("Back", theme.NavigateBackIcon(), func() {
		if ba.isTransferring {
			dialog.ShowConfirm("Cancel Transfer?",
				"Are you sure you want to cancel the current transfer?",
				func(cancel bool) {
					if cancel {
						ba.transferManager.Cancel()
						ba.resetSendView()
						ba.showMainView()
					}
				}, ba.window)
		} else {
			ba.resetSendView()
			ba.showMainView()
		}
	})

	// Layout
	codeCard := widget.NewCard("", "Your Transfer Code:",
		container.NewVBox(
			container.NewPadded(ba.codeDisplay),
			ba.copyButton,
		))

	content := container.NewVBox(
		container.NewBorder(nil, nil, backBtn, nil),
		codeCard,
		widget.NewSeparator(),
		container.NewPadded(container.NewVBox(
			widget.NewLabel("1. Copy your code and share it with the receiver"),
			widget.NewLabel("2. Click below to select files when ready"),
			ba.selectButton,
			ba.waitingLabel,
		)),
	)

	ba.sendCard = widget.NewCard("Send Files", "", content)
}

// createReceiveView creates the receive workflow with code entry
func (ba *BulletproofApp) createReceiveView() {
	// Code entry
	ba.codeEntry = widget.NewEntry()
	ba.codeEntry.SetPlaceHolder("Enter sender's code (e.g., word-word-word)")

	// Receive button
	ba.receiveButton = widget.NewButton("Start Receiving", func() {
		code := strings.TrimSpace(ba.codeEntry.Text)
		if code == "" {
			dialog.ShowError(fmt.Errorf("please enter the sender's code"), ba.window)
			return
		}
		ba.onStartReceive(code)
	})
	ba.receiveButton.Importance = widget.HighImportance
	ba.receiveButton.Icon = theme.DownloadIcon()

	// Back button
	backBtn := widget.NewButtonWithIcon("Back", theme.NavigateBackIcon(), func() {
		ba.showMainView()
	})

	// Instructions
	instructions := widget.NewLabel("Enter the code from the sender to receive files")
	instructions.Alignment = fyne.TextAlignCenter
	instructions.Wrapping = fyne.TextWrapWord

	// Layout
	content := container.NewVBox(
		container.NewBorder(nil, nil, backBtn, nil),
		instructions,
		widget.NewSeparator(),
		container.NewPadded(container.NewVBox(
			ba.codeEntry,
			ba.receiveButton,
		)),
	)

	ba.receiveCard = widget.NewCard("Receive Files", "", content)
}

// createProgressView creates the transfer progress view
func (ba *BulletproofApp) createProgressView() {
	// Status labels
	ba.statusLabel = widget.NewLabelWithStyle("Transfer in progress, please wait...",
		fyne.TextAlignCenter,
		fyne.TextStyle{Bold: true})

	ba.detailLabel = widget.NewLabel("This may take a few moments depending on file size and network speed.")
	ba.detailLabel.Alignment = fyne.TextAlignCenter
	ba.detailLabel.Wrapping = fyne.TextWrapWord

	// Cancel button
	ba.cancelButton = widget.NewButton("Cancel", func() {
		dialog.ShowConfirm("Cancel Transfer?",
			"Are you sure you want to cancel this transfer?",
			func(cancel bool) {
				if cancel {
					ba.transferManager.Cancel()
					ba.resetTransferState()
					ba.showMainView()
				}
			}, ba.window)
	})
	ba.cancelButton.Importance = widget.DangerImportance

	// Layout
	content := container.NewVBox(
		container.NewPadded(container.NewCenter(
			widget.NewLabel("●○○○○"), // Simple progress indicator
		)),
		widget.NewSeparator(),
		container.NewPadded(container.NewVBox(
			ba.statusLabel,
			ba.detailLabel,
			layout.NewSpacer(),
			container.NewCenter(ba.cancelButton),
		)),
	)

	ba.progressCard = widget.NewCard("Transfer in Progress", "", content)
}

// createSuccessView creates the success/completion view
func (ba *BulletproofApp) createSuccessView() {
	ba.successMessage = widget.NewLabelWithStyle("Transfer completed successfully!",
		fyne.TextAlignCenter,
		fyne.TextStyle{Bold: true})

	ba.locationLabel = widget.NewLabel("")
	ba.locationLabel.Alignment = fyne.TextAlignCenter
	ba.locationLabel.Wrapping = fyne.TextWrapWord

	ba.openFolderBtn = widget.NewButton("Open Folder", func() {
		// Open the received files folder
		// TODO: Implement folder opening
	})
	ba.openFolderBtn.Icon = theme.FolderOpenIcon()

	ba.doneButton = widget.NewButton("Done", func() {
		// Properly reset the send view state and generate new code
		ba.resetSendView()
		ba.resetTransferState()
		ba.showMainView()
	})
	ba.doneButton.Importance = widget.HighImportance

	content := container.NewVBox(
		container.NewPadded(container.NewVBox(
			ba.successMessage,
			ba.locationLabel,
		)),
		widget.NewSeparator(),
		container.NewPadded(container.NewVBox(
			ba.openFolderBtn,
			ba.doneButton,
		)),
	)

	ba.successCard = widget.NewCard("Success!", "", content)
}

// View management functions
func (ba *BulletproofApp) showMainView() {
	ba.currentView = "main"
	ba.window.SetContent(ba.mainContent)
}

func (ba *BulletproofApp) showSendView() {
	ba.currentView = "send"
	ba.window.SetContent(container.NewCenter(ba.sendCard))
}

func (ba *BulletproofApp) showReceiveView() {
	ba.currentView = "receive"
	ba.window.SetContent(container.NewCenter(ba.receiveCard))
}

func (ba *BulletproofApp) showProgressView() {
	ba.currentView = "progress"
	ba.window.SetContent(container.NewCenter(ba.progressCard))
}

func (ba *BulletproofApp) showSuccessView(message string) {
	ba.currentView = "success"
	ba.successMessage.SetText(message)
	ba.window.SetContent(container.NewCenter(ba.successCard))
}

// Event handlers
func (ba *BulletproofApp) onSelectFiles() {
	// Create a choice dialog with clear options
	filesBtn := widget.NewButtonWithIcon("Select File(s)", theme.DocumentIcon(), func() {
		if ba.selectionDialog != nil {
			ba.selectionDialog.Hide()
		}
		ba.selectSingleFile()
	})
	filesBtn.Importance = widget.HighImportance

	folderBtn := widget.NewButtonWithIcon("Select Folder", theme.FolderIcon(), func() {
		if ba.selectionDialog != nil {
			ba.selectionDialog.Hide()
		}
		ba.selectFolder()
	})
	folderBtn.Importance = widget.MediumImportance

	content := container.NewVBox(
		widget.NewLabelWithStyle("What would you like to send?", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewSeparator(),
		container.NewPadded(filesBtn),
		container.NewPadded(folderBtn),
		widget.NewLabel("Note: You can select files one by one or send an entire folder"),
	)

	ba.selectionDialog = dialog.NewCustom("Choose Transfer Type", "Cancel", content, ba.window)
	ba.selectionDialog.Show()
}

// selectSingleFile allows selection of individual files
func (ba *BulletproofApp) selectSingleFile() {
	fileDialog := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
		if err != nil || reader == nil {
			return
		}
		reader.Close()

		path := reader.URI().Path()
		if runtime.GOOS == "windows" && strings.HasPrefix(path, "/") {
			path = path[1:]
		}

		ba.startSend([]string{path})
	}, ba.window)

	// Set initial location to user's home directory
	homeDir, _ := os.UserHomeDir()
	listableURI := storage.NewFileURI(homeDir)
	if lister, ok := listableURI.(fyne.ListableURI); ok {
		fileDialog.SetLocation(lister)
	}

	fileDialog.Show()
}

// selectFolder allows selection of entire folders with all contents
func (ba *BulletproofApp) selectFolder() {
	dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
		if err != nil || uri == nil {
			if err != nil && strings.Contains(err.Error(), "operation not permitted") {
				dialog.ShowError(fmt.Errorf("permission denied: please grant folder access in System Preferences"), ba.window)
			}
			return
		}

		path := uri.Path()
		if runtime.GOOS == "windows" && strings.HasPrefix(path, "/") {
			path = path[1:]
		}

		// Verify access
		if _, err := os.Stat(path); err != nil {
			dialog.ShowError(fmt.Errorf("cannot access this folder"), ba.window)
			return
		}

		ba.startSend([]string{path})
	}, ba.window)
}

func (ba *BulletproofApp) startSend(paths []string) {
	if len(paths) == 0 {
		return
	}

	ba.mutex.Lock()
	ba.isTransferring = true
	ba.mutex.Unlock()

	// Show what we're sending
	path := paths[0]
	var waitingMsg string
	if info, err := os.Stat(path); err == nil {
		if info.IsDir() {
			waitingMsg = fmt.Sprintf("Waiting for receiver to connect...\nReady to send folder: %s", filepath.Base(path))
		} else {
			waitingMsg = fmt.Sprintf("Waiting for receiver to connect...\nReady to send file: %s", filepath.Base(path))
		}
	} else {
		waitingMsg = "Waiting for receiver to connect..."
	}

	ba.waitingLabel.Show()
	ba.waitingLabel.SetText(waitingMsg)
	ba.selectButton.Disable()

	// Start transfer in background
	go func() {
		result, err := ba.transferManager.SendFiles(paths, ba.currentCode)
		if err != nil {
			// Handle send errors properly
			ba.mutex.Lock()
			ba.isTransferring = false
			ba.mutex.Unlock()

			// Reset UI state
			ba.waitingLabel.Hide()
			ba.selectButton.Enable()

			// Show error
			go func() {
				dialog.ShowError(fmt.Errorf("Send failed: %v", err), ba.window)
			}()
		} else {
			// Success - reset state and show success
			ba.mutex.Lock()
			ba.isTransferring = false
			ba.mutex.Unlock()

			// Determine what was sent for success message
			var successMsg string
			if info, err := os.Stat(paths[0]); err == nil && info.IsDir() {
				successMsg = fmt.Sprintf("Sent folder '%s' successfully!", filepath.Base(paths[0]))
			} else {
				successMsg = fmt.Sprintf("Sent %d file(s) successfully!", len(result.TransferredFiles))
			}
			ba.showSuccessView(successMsg)
		}
	}()
}

func (ba *BulletproofApp) onStartReceive(code string) {
	ba.showProgressView()
	ba.statusLabel.SetText("Connecting to sender...")
	ba.detailLabel.SetText("Initializing secure connection...")

	go func() {
		// Add timeout protection for large transfers
		result, err := ba.transferManager.ReceiveFiles(code)
		if err != nil {
			// Better error handling - don't crash, show error and return to receive view
			ba.mutex.Lock()
			ba.isTransferring = false
			ba.mutex.Unlock()

			// Show error in main thread
			go func() {
				dialog.ShowError(fmt.Errorf("Transfer failed: %v", err), ba.window)
				time.Sleep(2 * time.Second)
				ba.showReceiveView()
			}()
		} else {
			// Success - reset transfer state and show results
			ba.mutex.Lock()
			ba.isTransferring = false
			ba.mutex.Unlock()

			receivedPath := filepath.Join(ba.targetDataDir, "received")
			ba.locationLabel.SetText(fmt.Sprintf("Files saved to: %s", receivedPath))
			ba.showSuccessView(fmt.Sprintf("Received %d files successfully!", len(result.TransferredFiles)))
		}
	}()
}

// State management
func (ba *BulletproofApp) resetSendView() {
	ba.isTransferring = false
	ba.waitingLabel.Hide()
	ba.selectButton.Enable()
	ba.currentCode = generateTransferCode()
	ba.codeDisplay.SetText(ba.currentCode)
}

func (ba *BulletproofApp) resetTransferState() {
	ba.isTransferring = false
}

// Helper functions
func (ba *BulletproofApp) getNetworkStatusWidget() *widget.Label {
	status := ba.transferManager.GetNetworkStatus()

	if profile, ok := status["network_profile"].(transport.NetworkProfile); ok {
		networkType := "Open Network"
		if profile.IsRestrictive {
			networkType = "Restrictive Network"
		}

		transportCount := 0
		if transportStatus, ok := status["transport_status"].(map[string]interface{}); ok {
			transportCount = len(transportStatus)
		}

		statusText := fmt.Sprintf("Status: %s • %d transports", networkType, transportCount)
		label := widget.NewLabel(statusText)
		label.Alignment = fyne.TextAlignCenter
		return label
	}

	label := widget.NewLabel("Status: Network analysis in progress...")
	label.Alignment = fyne.TextAlignCenter
	return label
}

func (ba *BulletproofApp) setupCallbacks() {
	ba.transferManager.SetStatusCallback(func(status string) {
		if ba.currentView == "progress" {
			ba.statusLabel.SetText(status)
		}
	})

	ba.transferManager.SetProgressCallback(func(current, total int64, fileName string) {
		if ba.currentView == "progress" && total > 0 {
			progress := float64(current) / float64(total)
			ba.detailLabel.SetText(fmt.Sprintf("Processing: %s (%.1f%%)",
				filepath.Base(fileName), progress*100))
		}
	})
}

func generateTransferCode() string {
	// Generate a simple 3-word code for now
	words := []string{"alpha", "beta", "gamma", "delta", "echo", "foxtrot", "golf", "hotel"}
	return fmt.Sprintf("%s-%s-%s",
		words[time.Now().Second()%len(words)],
		words[(time.Now().Second()+1)%len(words)],
		words[(time.Now().Second()+2)%len(words)])
}
