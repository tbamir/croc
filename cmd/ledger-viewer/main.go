package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"text/tabwriter"
	"trustdrop/logging"
)

func main() {
	var (
		verify = flag.Bool("verify", false, "Verify blockchain integrity")
		export = flag.String("export", "", "Export blockchain to JSON file")
		view   = flag.Bool("view", false, "View transfer history")
		blocks = flag.Bool("blocks", false, "View raw blockchain blocks")
	)

	flag.Parse()

	// Initialize logger (which includes blockchain)
	logger, err := logging.NewLogger()
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}

	// Handle different commands
	switch {
	case *verify:
		verifyLedger(logger)
	case *export != "":
		exportLedger(logger, *export)
	case *view:
		viewTransfers(logger)
	case *blocks:
		viewBlocks(logger)
	default:
		fmt.Println("TrustDrop Ledger Viewer")
		fmt.Println("======================")
		fmt.Println()
		fmt.Println("Usage:")
		fmt.Println("  ledger-viewer -verify          Verify blockchain integrity")
		fmt.Println("  ledger-viewer -export <file>   Export blockchain to JSON")
		fmt.Println("  ledger-viewer -view            View transfer history")
		fmt.Println("  ledger-viewer -blocks          View raw blockchain blocks")
	}
}

func verifyLedger(logger *logging.Logger) {
	fmt.Println("Verifying blockchain integrity...")
	
	valid, err := logger.VerifyLedger()
	if err != nil {
		fmt.Printf("❌ Verification failed: %v\n", err)
		os.Exit(1)
	}
	
	if valid {
		fmt.Println("✅ Blockchain is valid and intact!")
		
		info := logger.GetBlockchainInfo()
		fmt.Printf("\nBlockchain Statistics:\n")
		fmt.Printf("  Total Blocks: %d\n", info["chain_length"])
		
		if latestBlock, ok := info["latest_block"].(map[string]interface{}); ok {
			fmt.Printf("  Latest Block Index: %d\n", latestBlock["index"])
			fmt.Printf("  Latest Block Hash: %s\n", latestBlock["hash"])
		}
	} else {
		fmt.Println("❌ Blockchain integrity compromised!")
		os.Exit(1)
	}
}

func exportLedger(logger *logging.Logger, filename string) {
	fmt.Printf("Exporting blockchain to %s...\n", filename)
	
	err := logger.ExportLedger(filename)
	if err != nil {
		fmt.Printf("❌ Export failed: %v\n", err)
		os.Exit(1)
	}
	
	fmt.Printf("✅ Blockchain exported successfully to %s\n", filename)
}

func viewTransfers(logger *logging.Logger) {
	logs, err := logger.GetLogs()
	if err != nil {
		fmt.Printf("❌ Failed to get logs: %v\n", err)
		os.Exit(1)
	}
	
	if len(logs) == 0 {
		fmt.Println("No transfers found in blockchain.")
		return
	}
	
	fmt.Printf("\nTransfer History (%d transfers):\n", len(logs))
	fmt.Println("=====================================")
	
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "Time\tDirection\tPeer ID\tFile\tSize\tStatus\tDuration")
	fmt.Fprintln(w, "----\t---------\t-------\t----\t----\t------\t--------")
	
	for _, log := range logs {
		timestamp := log.Timestamp.Format("2006-01-02 15:04:05")
		size := formatBytes(log.FileSize)
		
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			timestamp,
			log.Direction,
			truncate(log.PeerID, 15),
			truncate(log.FileName, 25),
			size,
			log.Status,
			log.Duration,
		)
	}
	w.Flush()
}

func viewBlocks(logger *logging.Logger) {
	// This requires direct blockchain access
	// We'll need to expose a method to get the blockchain instance
	fmt.Println("Raw blockchain blocks:")
	fmt.Println("=====================")
	
	info := logger.GetBlockchainInfo()
	fmt.Printf("\nTotal blocks: %d\n", info["chain_length"])
	fmt.Println("\nNote: For detailed block view, export the blockchain and examine the JSON file.")
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}