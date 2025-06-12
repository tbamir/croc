# TrustDrop Bulletproof Edition

**Enterprise-grade P2P file transfer for Europe-to-US transfers through corporate/university firewalls**

## 🎯 What is TrustDrop Bulletproof Edition?

TrustDrop Bulletproof Edition is an advanced file transfer application specifically designed for secure, reliable transfers between Europe and US laboratories, corporate networks, and university environments. It features multiple transport protocols optimized for restrictive network environments.

## ✨ Key Features

- **🔥 Firewall Traversal**: Multiple transport methods (HTTPS, WebSocket, CROC P2P)
- **🌍 International Ready**: Optimized for Europe-to-US transfers
- **🏢 Corporate Network Compatible**: Works through proxies and restrictive firewalls
- **🔐 Enterprise Security**: Cryptographically secure with advanced encryption
- **📱 User-Friendly GUI**: Intuitive interface with network status monitoring
- **🚫 No Authentication Required**: Primary transports work out-of-the-box
- **⚡ Automatic Failover**: Smart transport selection based on network conditions

## 🚀 Transport Technologies

1. **HTTPS International (Priority 95)** - Uses public HTTPS services (Transfer.sh, File.io, 0x0.st, GitHub Gists)
2. **WebSocket (Priority 70)** - Firewall-friendly WebSocket echo services
3. **CROC P2P (Priority 60)** - Peer-to-peer with international relay servers
4. **Tor (Priority 50)** - Maximum privacy (optional, requires Tor installation)

## 📦 Downloads

### macOS
- **File**: `TrustDrop-Bulletproof.app`
- **Requirements**: macOS 10.15 (Catalina) or later
- **Installation**: Drag to `/Applications` folder

### Windows
- **File**: `TrustDrop-Bulletproof.exe`
- **Requirements**: Windows 10 or later
- **Installation**: No installation required - portable executable

## 🛠 Installation Instructions

### macOS Installation

1. **Download** `TrustDrop-Bulletproof.app` from the releases
2. **Drag** the app to your `/Applications` folder
3. **Right-click** the app and select "Open" (first time only)
4. If macOS says "app is damaged", run this command in Terminal:
   ```bash
   sudo xattr -d com.apple.quarantine /Applications/TrustDrop-Bulletproof.app
   ```

### Windows Installation

1. **Download** `TrustDrop-Bulletproof.exe` from the releases
2. **Run** the executable (no installation required)
3. If Windows Defender blocks it, add to exclusions:
   - Windows Security → Virus & threat protection → Exclusions → Add exclusion
   - Choose "File" and select `TrustDrop-Bulletproof.exe`

## 🎮 How to Use

### Sending Files

1. **Click** "Send Files" in the main interface
2. **Select** files or folders to transfer
3. **Share** the generated transfer code with the recipient
4. **Wait** for the transfer to complete

### Receiving Files

1. **Click** "Receive Files" in the main interface
2. **Enter** the transfer code provided by the sender
3. **Click** "Start Receive"
4. Files will be saved to `Documents/TrustDrop Downloads/`

## 🌐 Network Compatibility

### ✅ Works Great With:
- Corporate firewalls and proxies
- University networks with restrictions
- Hotel and conference center WiFi
- VPN connections
- Most restrictive network environments

### ⚠️ May Need Assistance:
- Air-gapped networks (no internet)
- Networks blocking all external domains
- Extremely restrictive DPI systems

## 🔧 Troubleshooting

### Common Issues

**"App is damaged" (macOS)**
```bash
sudo xattr -d com.apple.quarantine /Applications/TrustDrop-Bulletproof.app
```

**Antivirus blocking (Windows)**
- Add `TrustDrop-Bulletproof.exe` to antivirus exclusions
- The app needs network access for file transfers

**Transfer fails repeatedly**
- Check network status in the app
- Try different network (mobile hotspot)
- Ensure both devices have internet access

**Slow transfers**
- Large files may take time through HTTPS services
- Corporate networks may have bandwidth limits
- Use during off-peak hours for better performance

### Network Status Indicators

- 🌐 **Open Network**: Optimal conditions
- 🏢 **Corporate Network**: May have restrictions but should work
- 🎓 **University Network**: Optimized configuration active
- 🔒 **Restricted Network**: Limited options, fallback transports active

## 🔒 Security Features

- **End-to-end encryption** for all transfers
- **Cryptographically secure** PAKE key exchange
- **Memory protection** against large file attacks
- **No data retention** on relay servers
- **Automatic cleanup** of temporary files

## 📋 File Size Limits

- **WebSocket**: 512KB (for small files)
- **GitHub Gists**: 25MB (requires GitHub token for larger files)
- **File.io**: 2GB
- **Transfer.sh**: 10GB
- **CROC P2P**: No practical limit (direct P2P)

## 🌍 International Usage

**Europe → US Transfers:**
- Optimized relay server selection
- Multiple geographic endpoints
- Automatic failover between services
- Extended timeouts for international latency

**Tested Networks:**
- European university networks
- US corporate environments
- International VPN connections
- Hotel and conference WiFi

## 🏗 Building from Source

### macOS Build
```bash
./build.sh
```

### Windows Build
```batch
build.bat
```

**Requirements:**
- Go 1.21 or later
- Git
- ImageMagick (optional, for icon processing)

## 📞 Support

If you encounter issues:

1. Check the network status indicator in the app
2. Try a different network connection
3. Verify both sender and receiver have internet access
4. For corporate networks, contact IT if all transports fail

## 🔄 Version History

**v1.0.0 - Bulletproof Edition**
- Complete security audit and fixes
- Multiple transport protocols
- Corporate network optimization
- Advanced firewall traversal
- International relay configuration

## 📄 License

Copyright (C) 2024 TrustDrop. All rights reserved.

## 🚨 Important Notes

- **First Run**: macOS may show security warnings - this is normal
- **Corporate Networks**: App is designed to work with most corporate firewalls
- **Privacy**: No user data is stored or transmitted beyond the transferred files
- **Internet Required**: Both sender and receiver need internet access
- **File Persistence**: Files on relay services are automatically cleaned up 