# 🏢 TrustDrop Corporate/University Environment Setup

## 🎯 **For Europe-to-US Large File Transfers Through Firewalls**

### **1. Environment Variables (CRITICAL)**

#### **For HTTPS Transport (Recommended)**
```bash
# Set GitHub Personal Access Token for large file support
export GITHUB_TOKEN="ghp_your_personal_access_token_here"

# Corporate proxy settings (if required)
export HTTP_PROXY="http://proxy.company.com:8080"
export HTTPS_PROXY="http://proxy.company.com:8080"
export NO_PROXY="localhost,127.0.0.1,.company.com"
```

#### **Windows PowerShell**
```powershell
# Set GitHub token
$env:GITHUB_TOKEN="ghp_your_personal_access_token_here"

# Corporate proxy
$env:HTTP_PROXY="http://proxy.company.com:8080"
$env:HTTPS_PROXY="http://proxy.company.com:8080"
```

### **2. GitHub Personal Access Token Setup**

1. **Go to**: https://github.com/settings/tokens
2. **Click**: "Generate new token (classic)"
3. **Scopes**: Select `gist` (create gists)
4. **Copy token** and set as `GITHUB_TOKEN` environment variable

### **3. Corporate Firewall Configuration**

#### **Required Outbound Ports (Whitelist These)**
```
Port 443 (HTTPS) - Primary transport
Port 80 (HTTP) - Secondary transport  
Port 9009-9013 (TCP) - CROC relay fallback
```

#### **Required Domains (Whitelist These)**
```
api.github.com - GitHub Gists API
gist.github.com - Gist hosting
croc.schollz.com - CROC relay server
ws.postman-echo.com - WebSocket fallback
echo.websocket.org - WebSocket fallback
```

### **4. University Lab Configuration**

#### **Network Administrator Instructions**
```bash
# Allow outbound HTTPS to GitHub
iptables -A OUTPUT -p tcp --dport 443 -d api.github.com -j ACCEPT
iptables -A OUTPUT -p tcp --dport 443 -d gist.github.com -j ACCEPT

# Allow CROC relay (if P2P needed)
iptables -A OUTPUT -p tcp --dport 443 -d croc.schollz.com -j ACCEPT
iptables -A OUTPUT -p tcp --dport 80 -d croc.schollz.com -j ACCEPT
```

### **5. Testing Corporate Connectivity**

#### **Test Script (Run Before Transfer)**
```bash
#!/bin/bash
echo "🧪 Testing TrustDrop Corporate Connectivity..."

# Test GitHub API
curl -s -o /dev/null -w "%{http_code}" https://api.github.com/gists
echo "GitHub API: $?"

# Test CROC relay
nc -zv croc.schollz.com 443
echo "CROC 443: $?"

nc -zv croc.schollz.com 80  
echo "CROC 80: $?"

# Test WebSocket
curl -s -o /dev/null -w "%{http_code}" https://ws.postman-echo.com
echo "WebSocket: $?"
```

### **6. Large File Transfer Optimization**

#### **File Size Recommendations**
- **< 50MB**: HTTPS Transport (fastest through firewalls)
- **50MB - 5GB**: CROC Transport (reliable for large files)
- **> 5GB**: Split into chunks or use CROC with compression

#### **Transfer Strategy**
1. **Primary**: HTTPS via GitHub Gists (corporate-friendly)
2. **Secondary**: CROC via ports 443/80 (firewall-optimized)
3. **Fallback**: WebSocket (if available)

### **7. Troubleshooting Corporate Issues**

#### **Common Problems & Solutions**

**Problem**: "GitHub API 401 Authentication"
**Solution**: Set `GITHUB_TOKEN` environment variable

**Problem**: "Connection refused on port 9009"
**Solution**: Firewall blocking P2P ports - HTTPS will auto-fallback

**Problem**: "Proxy authentication required"
**Solution**: Set proxy credentials in environment variables

**Problem**: "SSL certificate verification failed"
**Solution**: Corporate firewall doing SSL inspection - contact IT

### **8. Production Deployment**

#### **Recommended Corporate Settings**
```bash
# Launch TrustDrop with corporate optimizations
export TRUSTDROP_MODE="corporate"
export TRUSTDROP_TIMEOUT="300"  # 5 minutes for international transfers
export TRUSTDROP_RETRY_COUNT="3"
export TRUSTDROP_PREFERRED_TRANSPORT="https"

./trustdrop-bulletproof
```

### **9. Security Compliance**

#### **Enterprise Security Features**
- ✅ **AES-256 encryption** (all transports)
- ✅ **TLS 1.2+ only** (HTTPS transport)
- ✅ **No local P2P** (corporate mode)
- ✅ **Audit trail logging** (blockchain-based)
- ✅ **Proxy support** (corporate networks)
- ✅ **No data persistence** (memory-only processing)

### **10. Performance Optimization**

#### **Europe-to-US Transfer Settings**
```bash
# Optimize for international latency
export TRUSTDROP_CONNECT_TIMEOUT="120"
export TRUSTDROP_READ_TIMEOUT="300" 
export TRUSTDROP_CHUNK_SIZE="1048576"  # 1MB chunks
export TRUSTDROP_CONCURRENT_CHUNKS="4"
```

---

## 🚀 **Ready for Mission-Critical Corporate Transfers!**

Your TrustDrop Bulletproof Edition is now configured for:
- ✅ **Corporate firewall traversal**
- ✅ **University network compliance** 
- ✅ **Europe-to-US optimization**
- ✅ **Large file reliability**
- ✅ **Enterprise security standards** 