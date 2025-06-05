# TrustDrop

A secure peer-to-peer file transfer application built with Go.

## Overview

TrustDrop enables secure peer-to-peer file transfers with strong encryption and no reliance on external servers for storage. Files are transferred directly between peers using the proven croc protocol with a simple, intuitive desktop GUI.

**Key Features:**
- Secure peer-to-peer connection using croc protocol
- End-to-end encryption for all transferred files
- TLS transmission with certificate verification
- Simple and intuitive desktop user interface
- Cross-platform support (macOS and Windows)
- Transfer audit logging
- Support for large file transfers (20,000+ files, terabytes of data)
- Automatic retry and resume capabilities

## Installation

### Prerequisites
- Go 1.24 or higher
- Git

### Building from Source

1. Clone the repository:
```bash
git clone <repository-url>
cd trustdrop
```

2. Build the application:
```bash
go build
```

3. Run the application:
```bash
./trustdrop
```

## Usage Guide

### Connecting to Peers

1. **Share your Peer ID**:
   - Generate a unique peer ID (code phrase) to share with your recipient
   - Share it via a secure channel (e.g., messaging app, email)

2. **Connect to a Peer**:
   - Enter the Peer ID of your recipient in the "Peer ID" field
   - Click "Connect" to establish a connection
   - Wait for the connection status to show "Connected"

### Sending Files

1. **Select Files or Folders**:
   - Click "Select Files/Folders"
   - Choose the files or folder you want to send

2. **Initiate Transfer**:
   - Click "Send" to begin the transfer
   - The progress bar will show the current transfer status
   - Transfer occurs in the background

3. **Transfer Complete**:
   - You'll be notified when the transfer is complete
   - Files are automatically encrypted during transfer
   - Received files are stored in the `data/received/` directory

### Viewing Audit Logs

- Transfer logs are automatically stored in the `logs/` directory
- Logs contain: date/time, file name, size, peer ID, result (success/failure), and any errors

## Testing Between Two Machines

1. **Setup Both Machines**:
   - Build and run TrustDrop on both machines

2. **Exchange Peer IDs**:
   - Generate a peer ID on the sending machine
   - Share the peer ID with the receiving machine using a separate communication channel

3. **Establish Connection**:
   - On receiving machine: Enter the peer ID and click "Connect"
   - Wait for connection to establish

4. **Test Transfer**:
   - On sending machine: Select files to send
   - Files will be received automatically and stored in `data/received/`

5. **Verify Transfer**:
   - Check the received files on the receiving machine
   - Verify the audit logs in the `logs/` directory

## Project Structure

```
trustdrop/
├── main.go                  // Application entry point
├── go.mod                   // Go module dependencies
├── README.md               // This file
├── assets/                 // UI assets (icons, etc.)
├── gui/                    // GUI implementation (Fyne-based)
├── transfer/               // File transfer logic (wraps croc)
├── security/               // Additional encryption and security
├── logging/                // Transfer audit logging
├── internal/               // Shared utilities
└── data/                   // Runtime data directory
    ├── received/           // Received files storage
    └── temp/              // Temporary files
```

## Security

TrustDrop implements several security measures:

- End-to-end encryption using the croc protocol
- AES-256-CBC encryption capabilities for additional security
- TLS transmission with certificate verification
- SHA-256 file integrity verification
- All actions are logged for audit purposes

## Troubleshooting

### Connection Issues

If you encounter connection problems:

1. **Check Network Configuration**:
   - Ensure both peers have internet connectivity
   - The application uses relay servers for NAT traversal

2. **Try Again**:
   - Sometimes connections can take a moment to establish
   - Disconnect and try connecting again

3. **Verify Peer ID**:
   - Double-check that you entered the correct Peer ID

### File Transfer Issues

If file transfers fail:

1. **Check Connection Status**:
   - Ensure you're still connected to the peer

2. **Check Disk Space**:
   - Ensure you have sufficient disk space for the incoming files

3. **Restart Application**:
   - In rare cases, restarting both applications can resolve issues

## Development

This application is built on top of the excellent [croc](https://github.com/schollz/croc) file transfer tool, providing a GUI wrapper around its proven P2P transfer capabilities.

### Dependencies

- [Fyne](https://fyne.io/) - Cross-platform GUI framework
- [croc](https://github.com/schollz/croc) - Secure file transfer protocol

## License

[Include your license information here]
