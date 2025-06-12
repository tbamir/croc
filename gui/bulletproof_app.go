package gui

import (
	"fmt"
	"os"
	"os/exec"
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

	"trustdrop-bulletproof/assets"
	"trustdrop-bulletproof/src/utils"
	"trustdrop-bulletproof/transfer"
	"trustdrop-bulletproof/transport"
)

// BulletproofApp provides an intuitive GUI for bulletproof file transfers with institutional network awareness
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
	errorCard    *widget.Card

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

	// Error elements
	errorMessage  *widget.Label
	errorDetails  *widget.Label
	retryButton   *widget.Button
	helpButton    *widget.Button
	backFromError *widget.Button

	// Network status elements
	networkStatusLabel *widget.Label
	networkStatusIcon  *widget.Label

	// State
	currentView     string
	mutex           sync.Mutex
	isTransferring  bool
	currentCode     string
	selectionDialog *dialog.CustomDialog
	lastError       error
	lastOperation   string // "send" or "receive"
	lastPaths       []string
	lastReceiveCode string
	networkInfo     NetworkInfo
}

// NetworkInfo holds current network status information
type NetworkInfo struct {
	Type                 string
	IsRestrictive        bool
	AvailableTransports  int
	RecommendedTransport string
	Restrictions         []string
	LastUpdated          time.Time
}

// NewAppWithBulletproofManager creates a new bulletproof app with enhanced UX and network awareness
func NewAppWithBulletproofManager(transferManager *transfer.BulletproofTransferManager, targetDataDir string) *BulletproofApp {
	bulletproofApp := &BulletproofApp{
		app:             app.New(),
		transferManager: transferManager,
		targetDataDir:   targetDataDir,
		currentCode:     generateTransferCode(),
		networkInfo: NetworkInfo{
			Type:                 "analyzing",
			IsRestrictive:        false,
			AvailableTransports:  0,
			RecommendedTransport: "https-tunnel",
			Restrictions:         []string{},
			LastUpdated:          time.Now(),
		},
	}

	bulletproofApp.setupUI()
	bulletproofApp.setupCallbacks()
	bulletproofApp.startNetworkMonitoring()

	return bulletproofApp
}

// Run starts the bulletproof application
func (ba *BulletproofApp) Run() {
	ba.window.ShowAndRun()
}

// setupUI creates the complete user interface with enhanced network awareness
func (ba *BulletproofApp) setupUI() {
	ba.window = ba.app.NewWindow("TrustDrop")
	ba.window.Resize(fyne.NewSize(560, 500)) // Larger for network status
	ba.window.CenterOnScreen()

	// Set the app icon from embedded assets
	ba.app.SetIcon(assets.GetAppIcon())
	ba.window.SetIcon(assets.GetAppIcon())

	// Create all views
	ba.createMainView()
	ba.createSendView()
	ba.createReceiveView()
	ba.createProgressView()
	ba.createSuccessView()
	ba.createErrorView()

	// Start with main view
	ba.showMainView()
}

// createMainView creates the main menu with comprehensive network status
func (ba *BulletproofApp) createMainView() {
	// App title
	title := widget.NewLabelWithStyle("TrustDrop Bulletproof",
		fyne.TextAlignCenter,
		fyne.TextStyle{Bold: true})

	subtitle := widget.NewLabel("Enterprise-grade P2P file transfer")
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

	// Network status section with detailed information
	networkStatus := ba.createNetworkStatusWidget()

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
		)),
		widget.NewSeparator(),
		networkStatus,
	)

	ba.mainContent = container.NewCenter(content)
}

// createNetworkStatusWidget creates a simple network status display
func (ba *BulletproofApp) createNetworkStatusWidget() *fyne.Container {
	ba.networkStatusIcon = widget.NewLabel("🔄") // Default: analyzing
	ba.networkStatusLabel = widget.NewLabel("Analyzing network environment...")
	ba.networkStatusLabel.Alignment = fyne.TextAlignCenter

	statusContainer := container.NewBorder(
		nil, nil,
		ba.networkStatusIcon,
		nil,
		ba.networkStatusLabel,
	)

	return container.NewVBox(
		widget.NewLabelWithStyle("Network Status", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		statusContainer,
	)
}

// startNetworkMonitoring monitors network status and updates UI
func (ba *BulletproofApp) startNetworkMonitoring() {
	go func() {
		time.Sleep(2 * time.Second) // Allow initial analysis

		for {
			status := ba.transferManager.GetNetworkStatus()
			ba.updateNetworkStatusDisplay(status)
			time.Sleep(15 * time.Second) // Update every 15 seconds
		}
	}()
}

// updateNetworkStatusDisplay updates the simplified network status display
func (ba *BulletproofApp) updateNetworkStatusDisplay(status map[string]interface{}) {
	var icon string
	var statusText string

	if profile, ok := status["network_profile"].(transport.NetworkProfile); ok {
		ba.networkInfo.Type = profile.NetworkType
		ba.networkInfo.IsRestrictive = profile.IsRestrictive
		ba.networkInfo.RecommendedTransport = profile.PreferredTransport
		ba.networkInfo.LastUpdated = time.Now()

		// Update main status
		if profile.IsRestrictive {
			switch profile.NetworkType {
			case "corporate":
				icon = "🏢"
				statusText = "Corporate Network"
			case "university":
				icon = "🎓"
				statusText = "University Network"
			case "institutional":
				icon = "🔒"
				statusText = "Institutional Network"
			default:
				icon = "🔒"
				statusText = "Restricted Network"
			}
		} else {
			icon = "🌐"
			statusText = "Open Network"
		}

		// Count available transport methods
		if transportStatus, ok := status["transport_status"].(map[string]interface{}); ok {
			availableCount := 0
			for _, transport := range transportStatus {
				if statusMap, ok := transport.(map[string]interface{}); ok {
					if available, ok := statusMap["available"].(bool); ok && available {
						availableCount++
					}
				}
			}
			ba.networkInfo.AvailableTransports = availableCount
			statusText += fmt.Sprintf(" • %d methods available", availableCount)
		}
	}

	// Update UI elements
	ba.networkStatusIcon.SetText(icon)
	ba.networkStatusLabel.SetText(statusText)
}

// createSendView creates the send workflow with enhanced network awareness
func (ba *BulletproofApp) createSendView() {
	// Code display with better formatting
	ba.codeDisplay = widget.NewLabelWithStyle(
		ba.currentCode,
		fyne.TextAlignCenter,
		fyne.TextStyle{Monospace: true, Bold: true})

	ba.copyButton = widget.NewButtonWithIcon("Copy Code", theme.ContentCopyIcon(), func() {
		ba.window.Clipboard().SetContent(ba.currentCode)
		ba.copyButton.SetText("Copied!")
		ba.copyButton.SetIcon(theme.ConfirmIcon())
		time.AfterFunc(2*time.Second, func() {
			ba.copyButton.SetText("Copy Code")
			ba.copyButton.SetIcon(theme.ContentCopyIcon())
		})
	})

	// Select files button
	ba.selectButton = widget.NewButton("Choose Files to Send", ba.onSelectFiles)
	ba.selectButton.Importance = widget.HighImportance
	ba.selectButton.Icon = theme.FolderOpenIcon()

	// Enhanced waiting indicator with network context
	ba.waitingLabel = widget.NewLabel("Share the code above with the receiver")
	ba.waitingLabel.Alignment = fyne.TextAlignCenter
	ba.waitingLabel.Wrapping = fyne.TextWrapWord
	ba.waitingLabel.Hide()

	// Network guidance label
	networkGuidanceLabel := widget.NewLabel("")
	networkGuidanceLabel.Alignment = fyne.TextAlignCenter
	networkGuidanceLabel.Wrapping = fyne.TextWrapWord

	// Update guidance based on current network
	ba.updateNetworkGuidance(networkGuidanceLabel)

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

	// Layout with network guidance
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
			widget.NewSeparator(),
			networkGuidanceLabel,
		)),
	)

	ba.sendCard = widget.NewCard("Send Files", "", content)
}

// updateNetworkGuidance updates network-specific guidance text
func (ba *BulletproofApp) updateNetworkGuidance(label *widget.Label) {
	var guidance string

	switch ba.networkInfo.Type {
	case "corporate":
		guidance = "Corporate Network: Using enterprise-friendly transfer methods that work through business firewalls and security systems."
	case "university":
		guidance = "University Network: Using education-network-compatible methods that respect academic IT policies."
	case "institutional":
		guidance = "Institutional Network: Using maximum compatibility mode designed for managed network environments."
	default:
		if ba.networkInfo.IsRestrictive {
			guidance = "Restricted Network: Automatically using institutional-compatible transfer methods for maximum reliability."
		} else {
			guidance = "Open Network: Using optimized transfer methods for best performance and security."
		}
	}

	if len(ba.networkInfo.Restrictions) > 0 {
		guidance += fmt.Sprintf(" (%d network restrictions detected and handled automatically)", len(ba.networkInfo.Restrictions))
	}

	label.SetText(guidance)
}

// createReceiveView creates the receive workflow with network awareness
func (ba *BulletproofApp) createReceiveView() {
	// Code entry with better UX
	ba.codeEntry = widget.NewEntry()
	ba.codeEntry.SetPlaceHolder("Enter sender's code (e.g., word-word-word)")
	ba.codeEntry.OnChanged = func(text string) {
		// Enable receive button only when code looks valid
		hasCode := len(strings.TrimSpace(text)) > 5
		ba.receiveButton.Enable()
		if !hasCode {
			ba.receiveButton.Disable()
		}
	}

	// Receive button
	ba.receiveButton = widget.NewButton("Start Receiving", func() {
		code := strings.TrimSpace(ba.codeEntry.Text)
		if code == "" {
			ba.showError("Invalid Code", "Please enter the sender's code", nil, false)
			return
		}
		ba.onStartReceive(code)
	})
	ba.receiveButton.Importance = widget.HighImportance
	ba.receiveButton.Icon = theme.DownloadIcon()
	ba.receiveButton.Disable() // Disabled until valid code entered

	// Back button
	backBtn := widget.NewButtonWithIcon("Back", theme.NavigateBackIcon(), func() {
		ba.showMainView()
	})

	// Network-aware instructions
	instructionsText := "**Enter the code from the sender to receive files**\n\n"
	if ba.networkInfo.IsRestrictive {
		instructionsText += "Institutional network detected - the app will automatically use compatible connection methods for your network environment."
	} else {
		instructionsText += "The app will automatically choose the best connection method for your network."
	}

	instructions := widget.NewRichTextFromMarkdown(instructionsText)
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

// createProgressView creates the enhanced transfer progress view with network context
func (ba *BulletproofApp) createProgressView() {
	// Status labels with better information hierarchy
	ba.statusLabel = widget.NewLabelWithStyle("Transfer in progress, please wait...",
		fyne.TextAlignCenter,
		fyne.TextStyle{Bold: true})

	ba.detailLabel = widget.NewLabel("Initializing secure connection...")
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

	// Network status during transfer
	networkStatusDuringTransfer := widget.NewLabel("")
	networkStatusDuringTransfer.Alignment = fyne.TextAlignCenter
	networkStatusDuringTransfer.Wrapping = fyne.TextWrapWord

	// Update with current network info
	ba.updateTransferNetworkStatus(networkStatusDuringTransfer)

	// Layout with enhanced spacing and network context
	content := container.NewVBox(
		container.NewPadded(container.NewCenter(
			widget.NewLabel("●●○○○"), // Progress indicator
		)),
		widget.NewSeparator(),
		container.NewPadded(container.NewVBox(
			ba.statusLabel,
			layout.NewSpacer(),
			ba.detailLabel,
			layout.NewSpacer(),
			networkStatusDuringTransfer,
			layout.NewSpacer(),
			container.NewCenter(ba.cancelButton),
		)),
	)

	ba.progressCard = widget.NewCard("Transfer in Progress", "", content)
}

// updateTransferNetworkStatus updates network status during transfer
func (ba *BulletproofApp) updateTransferNetworkStatus(label *widget.Label) {
	var statusText string

	if ba.networkInfo.IsRestrictive {
		statusText = fmt.Sprintf("Using %s transport for %s network compatibility",
			strings.Title(ba.networkInfo.RecommendedTransport), ba.networkInfo.Type)
	} else {
		statusText = fmt.Sprintf("Using optimized %s transport for best performance",
			strings.Title(ba.networkInfo.RecommendedTransport))
	}

	label.SetText(statusText)
}

// createSuccessView creates the success/completion view with enhanced information
func (ba *BulletproofApp) createSuccessView() {
	ba.successMessage = widget.NewLabelWithStyle("Transfer completed successfully!",
		fyne.TextAlignCenter,
		fyne.TextStyle{Bold: true})

	ba.locationLabel = widget.NewLabel("")
	ba.locationLabel.Alignment = fyne.TextAlignCenter
	ba.locationLabel.Wrapping = fyne.TextWrapWord

	ba.openFolderBtn = widget.NewButton("Open Folder", func() {
		ba.openReceivedFolder()
	})
	ba.openFolderBtn.Icon = theme.FolderOpenIcon()

	ba.doneButton = widget.NewButton("Done", func() {
		ba.resetSendView()
		ba.resetTransferState()
		ba.showMainView()
	})
	ba.doneButton.Importance = widget.HighImportance

	// Transfer summary
	transferSummary := widget.NewLabel("")
	transferSummary.Alignment = fyne.TextAlignCenter
	transferSummary.Wrapping = fyne.TextWrapWord

	content := container.NewVBox(
		container.NewPadded(container.NewVBox(
			container.NewCenter(widget.NewLabel("✅")), // Success icon
			ba.successMessage,
			ba.locationLabel,
			layout.NewSpacer(),
			transferSummary,
		)),
		widget.NewSeparator(),
		container.NewPadded(container.NewVBox(
			ba.openFolderBtn,
			ba.doneButton,
		)),
	)

	ba.successCard = widget.NewCard("Success!", "", content)
}

// createErrorView creates the enhanced error handling view with network troubleshooting
func (ba *BulletproofApp) createErrorView() {
	ba.errorMessage = widget.NewLabelWithStyle("Transfer Failed",
		fyne.TextAlignCenter,
		fyne.TextStyle{Bold: true})

	ba.errorDetails = widget.NewLabel("")
	ba.errorDetails.Alignment = fyne.TextAlignCenter
	ba.errorDetails.Wrapping = fyne.TextWrapWord

	// Retry button
	ba.retryButton = widget.NewButton("Try Again", func() {
		ba.retryLastOperation()
	})
	ba.retryButton.Importance = widget.HighImportance

	// Enhanced help button for network issues
	ba.helpButton = widget.NewButton("Network Help", func() {
		ba.showNetworkHelp()
	})
	ba.helpButton.Importance = widget.MediumImportance

	// Back button
	ba.backFromError = widget.NewButton("Back to Main", func() {
		ba.resetTransferState()
		ba.showMainView()
	})

	content := container.NewVBox(
		container.NewPadded(container.NewVBox(
			container.NewCenter(widget.NewLabel("❌")), // Error icon
			ba.errorMessage,
			layout.NewSpacer(),
			ba.errorDetails,
		)),
		widget.NewSeparator(),
		container.NewPadded(container.NewVBox(
			ba.retryButton,
			ba.helpButton,
			ba.backFromError,
		)),
	)

	ba.errorCard = widget.NewCard("Transfer Error", "", content)
}

// showError displays detailed error information with enhanced network context
func (ba *BulletproofApp) showError(title, message string, err error, showNetworkHelp bool) {
	ba.lastError = err
	ba.errorMessage.SetText(title)

	// Build detailed error message with network context
	var details strings.Builder
	details.WriteString(message)

	if err != nil {
		details.WriteString("\n\n")
		details.WriteString(err.Error())
	}

	// Add network context if error appears network-related
	if showNetworkHelp || ba.isNetworkRelatedError(err) {
		details.WriteString("\n\nNetwork Environment:")
		details.WriteString(fmt.Sprintf("\n• Type: %s", ba.networkInfo.Type))
		details.WriteString(fmt.Sprintf("\n• Restrictive: %t", ba.networkInfo.IsRestrictive))
		details.WriteString(fmt.Sprintf("\n• Available Methods: %d", ba.networkInfo.AvailableTransports))

		if len(ba.networkInfo.Restrictions) > 0 {
			details.WriteString(fmt.Sprintf("\n• Network Restrictions: %d detected", len(ba.networkInfo.Restrictions)))
		}
	}

	ba.errorDetails.SetText(details.String())

	// Show/hide network help based on error type
	if showNetworkHelp || ba.isNetworkRelatedError(err) {
		ba.helpButton.Show()
	} else {
		ba.helpButton.Hide()
	}

	ba.currentView = "error"
	ba.window.SetContent(container.NewCenter(ba.errorCard))
}

// isNetworkRelatedError checks if an error is network-related
func (ba *BulletproofApp) isNetworkRelatedError(err error) bool {
	if err == nil {
		return false
	}

	errorStr := strings.ToLower(err.Error())
	networkIndicators := []string{
		"network", "connection", "timeout", "refused", "unreachable",
		"firewall", "blocked", "restrictive", "institutional", "corporate",
		"university", "croc relay", "transport failed", "dial tcp",
		"proxy", "dns", "port", "managed network",
	}

	for _, indicator := range networkIndicators {
		if strings.Contains(errorStr, indicator) {
			return true
		}
	}

	return false
}

// showNetworkHelp displays comprehensive network troubleshooting information
func (ba *BulletproofApp) showNetworkHelp() {
	var helpText strings.Builder
	helpText.WriteString("Network Troubleshooting Guide\n\n")

	// Current network status
	helpText.WriteString(fmt.Sprintf("Current Network: %s", ba.networkInfo.Type))
	if ba.networkInfo.IsRestrictive {
		helpText.WriteString(" (Restrictive)")
	}
	helpText.WriteString("\n\n")

	// Network-specific guidance
	switch ba.networkInfo.Type {
	case "corporate":
		helpText.WriteString("🏢 Corporate Network Detected\n\n")
		helpText.WriteString("Your company network has security policies that may block peer-to-peer file transfers. ")
		helpText.WriteString("TrustDrop automatically uses enterprise-compatible methods, but some restrictions may still apply.\n\n")

		helpText.WriteString("Solutions:\n")
		helpText.WriteString("• Contact your IT department about approved file transfer methods\n")
		helpText.WriteString("• Try from a different network (mobile hotspot, home WiFi)\n")
		helpText.WriteString("• Use a personal device with mobile data if company policy permits\n")
		helpText.WriteString("• Ask IT about whitelisting TrustDrop's HTTPS-based transfer method\n")

	case "university":
		helpText.WriteString("🎓 University Network Detected\n\n")
		helpText.WriteString("Educational networks often have strict security policies for student safety and compliance. ")
		helpText.WriteString("TrustDrop uses education-friendly methods, but some restrictions may still apply.\n\n")

		helpText.WriteString("Solutions:\n")
		helpText.WriteString("• Try from the campus library or a different network zone\n")
		helpText.WriteString("• Use your mobile data connection if available\n")
		helpText.WriteString("• Contact IT support about student file transfer options\n")
		helpText.WriteString("• Try from an off-campus network (coffee shop, home)\n")

	case "institutional":
		helpText.WriteString("🔒 Institutional Network Detected\n\n")
		helpText.WriteString("This appears to be a managed network with comprehensive security policies. ")
		helpText.WriteString("TrustDrop is using maximum compatibility mode, but transfers may still be restricted.\n\n")

		helpText.WriteString("Solutions:\n")
		helpText.WriteString("• Contact network administrators about file transfer policies\n")
		helpText.WriteString("• Try from an unmanaged network (mobile hotspot, public WiFi)\n")
		helpText.WriteString("• Use organization-approved file sharing alternatives\n")
		helpText.WriteString("• Request temporary network access for file transfers\n")

	default:
		if ba.networkInfo.IsRestrictive {
			helpText.WriteString("🔒 Restricted Network Detected\n\n")
			helpText.WriteString("Your network has restrictions that may interfere with file transfers. ")
			helpText.WriteString("TrustDrop has automatically adjusted its methods for compatibility.\n\n")
		} else {
			helpText.WriteString("🌐 Open Network Detected\n\n")
			helpText.WriteString("Your network appears to be open, but transfers are still failing.\n\n")
		}

		helpText.WriteString("Solutions:\n")
		helpText.WriteString("• Check your internet connection stability\n")
		helpText.WriteString("• Verify the transfer code is correct\n")
		helpText.WriteString("• Try restarting your router/modem\n")
		helpText.WriteString("• Disable VPN temporarily if using one\n")
		helpText.WriteString("• Check if antivirus software is blocking connections\n")
	}

	// Transport method status
	helpText.WriteString("\nTransport Method Status:\n")
	if ba.networkInfo.AvailableTransports > 0 {
		helpText.WriteString(fmt.Sprintf("✅ %d transfer methods available\n", ba.networkInfo.AvailableTransports))
		helpText.WriteString(fmt.Sprintf("📡 Recommended: %s\n", strings.Title(ba.networkInfo.RecommendedTransport)))
	} else {
		helpText.WriteString("❌ No transfer methods currently available\n")
		helpText.WriteString("This indicates a severe network restriction or connectivity issue.\n")
	}

	// Network restrictions
	if len(ba.networkInfo.Restrictions) > 0 {
		helpText.WriteString("\nDetected Network Restrictions:\n")
		for i, restriction := range ba.networkInfo.Restrictions {
			if i < 3 { // Show max 3 restrictions to avoid clutter
				helpText.WriteString(fmt.Sprintf("• %s\n", restriction))
			}
		}
		if len(ba.networkInfo.Restrictions) > 3 {
			helpText.WriteString(fmt.Sprintf("• ... and %d more restrictions\n", len(ba.networkInfo.Restrictions)-3))
		}
	}

	// Additional help
	helpText.WriteString("\nAdditional Help:\n")
	helpText.WriteString("• Last network check: " + ba.networkInfo.LastUpdated.Format("15:04:05") + "\n")
	helpText.WriteString("• For persistent issues, try TrustDrop from a different device or network\n")
	helpText.WriteString("• Consider using your organization's approved file sharing platform\n")

	dialog.ShowInformation("Network Troubleshooting", helpText.String(), ba.window)
}

// retryLastOperation attempts to retry the last failed operation
func (ba *BulletproofApp) retryLastOperation() {
	if ba.lastOperation == "send" && len(ba.lastPaths) > 0 {
		ba.showProgressView()
		ba.startSend(ba.lastPaths)
	} else if ba.lastOperation == "receive" && ba.lastReceiveCode != "" {
		ba.showProgressView()
		ba.onStartReceive(ba.lastReceiveCode)
	} else {
		ba.showMainView()
	}
}

// openReceivedFolder opens the folder containing received files
func (ba *BulletproofApp) openReceivedFolder() {
	receivedPath := filepath.Join(ba.targetDataDir, "received")

	var cmd string
	var args []string

	switch runtime.GOOS {
	case "windows":
		cmd = "explorer"
		args = []string{receivedPath}
	case "darwin":
		cmd = "open"
		args = []string{receivedPath}
	case "linux":
		cmd = "xdg-open"
		args = []string{receivedPath}
	default:
		dialog.ShowInformation("Folder Location",
			fmt.Sprintf("Files saved to:\n%s", receivedPath), ba.window)
		return
	}

	// Try to open the folder
	if err := exec.Command(cmd, args...).Start(); err != nil {
		dialog.ShowInformation("Folder Location",
			fmt.Sprintf("Files saved to:\n%s\n\nCould not open folder automatically.", receivedPath),
			ba.window)
	}
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

// Enhanced event handlers with better error handling and network awareness
func (ba *BulletproofApp) onSelectFiles() {
	// Create a choice dialog with clear options and network context
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

	// Network context message
	var networkMsg string
	if ba.networkInfo.IsRestrictive {
		networkMsg = fmt.Sprintf("Note: %s network detected - using institutional-compatible transfer methods", ba.networkInfo.Type)
	} else {
		networkMsg = "Note: Using optimized transfer methods for your network"
	}

	content := container.NewVBox(
		widget.NewLabelWithStyle("What would you like to send?", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewSeparator(),
		container.NewPadded(filesBtn),
		container.NewPadded(folderBtn),
		widget.NewSeparator(),
		widget.NewLabel(networkMsg),
	)

	ba.selectionDialog = dialog.NewCustom("Choose Transfer Type", "Cancel", content, ba.window)
	ba.selectionDialog.Show()
}

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

func (ba *BulletproofApp) selectFolder() {
	dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
		if err != nil || uri == nil {
			if err != nil && strings.Contains(err.Error(), "operation not permitted") {
				ba.showError("Permission Denied",
					"Please grant folder access in System Preferences", err, false)
			}
			return
		}

		path := uri.Path()
		if runtime.GOOS == "windows" && strings.HasPrefix(path, "/") {
			path = path[1:]
		}

		// Verify access
		if _, err := os.Stat(path); err != nil {
			ba.showError("Access Error", "Cannot access this folder", err, false)
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
	ba.lastOperation = "send"
	ba.lastPaths = paths
	ba.mutex.Unlock()

	// Show what we're sending with network context
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

	// Add network-specific messaging
	if ba.networkInfo.IsRestrictive {
		waitingMsg += fmt.Sprintf("\n(Using %s-compatible transfer method)", ba.networkInfo.Type)
	}

	ba.waitingLabel.Show()
	ba.waitingLabel.SetText(waitingMsg)
	ba.selectButton.Disable()

	// Start transfer in background with enhanced error handling
	go func() {
		result, err := ba.transferManager.SendFiles(paths, ba.currentCode)

		ba.mutex.Lock()
		ba.isTransferring = false
		ba.mutex.Unlock()

		if err != nil {
			// Enhanced error handling with network context
			ba.waitingLabel.Hide()
			ba.selectButton.Enable()

			// Check if this is a network-related error
			if ba.isNetworkRelatedError(err) {
				ba.showError("Network Transfer Failed",
					"The transfer failed due to network restrictions or connectivity issues.",
					err, true) // Show network help
			} else {
				ba.showError("Transfer Failed",
					"The file transfer could not be completed.",
					err, false)
			}
		} else {
			// Success - show results with transfer details
			var successMsg string
			if info, err := os.Stat(paths[0]); err == nil && info.IsDir() {
				successMsg = fmt.Sprintf("Sent folder '%s' successfully!", filepath.Base(paths[0]))
			} else {
				successMsg = fmt.Sprintf("Sent %d file(s) successfully!", len(result.TransferredFiles))
			}

			// Update success view with transfer details
			ba.updateSuccessView(result)
			ba.showSuccessView(successMsg)
		}
	}()
}

func (ba *BulletproofApp) onStartReceive(code string) {
	ba.mutex.Lock()
	ba.lastOperation = "receive"
	ba.lastReceiveCode = code
	ba.mutex.Unlock()

	ba.showProgressView()
	ba.statusLabel.SetText("Connecting to sender...")
	ba.detailLabel.SetText("Analyzing network and choosing best connection method...")

	go func() {
		result, err := ba.transferManager.ReceiveFiles(code)

		ba.mutex.Lock()
		ba.isTransferring = false
		ba.mutex.Unlock()

		if err != nil {
			// Enhanced error handling for receive
			if ba.isNetworkRelatedError(err) {
				ba.showError("Network Connection Failed",
					"Could not connect to the sender due to network restrictions.",
					err, true)
			} else {
				ba.showError("Receive Failed",
					"Could not receive files from the sender.",
					err, false)
			}
		} else {
			// Success
			receivedPath := filepath.Join(ba.targetDataDir, "received")
			ba.locationLabel.SetText(fmt.Sprintf("Files saved to: %s", receivedPath))

			// Update success view with transfer details
			ba.updateSuccessView(result)
			ba.showSuccessView(fmt.Sprintf("Received %d files successfully!", len(result.TransferredFiles)))
		}
	}()
}

// updateSuccessView updates the success view with transfer details
func (ba *BulletproofApp) updateSuccessView(result *transfer.TransferResult) {
	if ba.successCard == nil || ba.successCard.Content == nil {
		return
	}

	// Find transfer summary label in success view
	if vbox, ok := ba.successCard.Content.(*fyne.Container); ok {
		if len(vbox.Objects) > 0 {
			if innerVBox, ok := vbox.Objects[0].(*fyne.Container); ok {
				// Add transfer summary if not already present
				if len(innerVBox.Objects) >= 3 {
					summaryText := fmt.Sprintf("Transfer Details:\n• Transport: %s\n• Duration: %v\n• Network: %s",
						strings.Title(result.TransportUsed),
						result.Duration.Round(time.Second),
						ba.networkInfo.Type)

					if len(innerVBox.Objects) > 3 {
						if label, ok := innerVBox.Objects[3].(*widget.Label); ok {
							label.SetText(summaryText)
						}
					}
				}
			}
		}
	}
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
	ba.lastError = nil
	ba.lastOperation = ""
	ba.lastPaths = nil
	ba.lastReceiveCode = ""
}

// setupCallbacks sets up transfer callbacks with enhanced progress reporting
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
	return utils.GetRandomName()
}
