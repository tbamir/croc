#!/bin/bash

# TrustDrop Firewall Simulation Test Script
# Simulates restrictive network conditions like Germany/Dallas corporate firewalls

echo "🔥 TrustDrop Firewall Simulation Test"
echo "===================================="
echo "Simulating Germany↔Dallas corporate firewall scenario"
echo ""

# Method 1: Block common P2P ports using pfctl (macOS) or iptables (Linux)
echo "🚫 Blocking common P2P ports (simulating corporate firewall)..."

if [[ "$OSTYPE" == "darwin"* ]]; then
    # macOS - Create firewall rules
    echo "Creating macOS firewall rules..."
    
    # Block croc default ports
    sudo pfctl -f /dev/stdin << 'EOF'
# Block P2P ports typically blocked by corporate firewalls
block drop out proto tcp from any to any port 9009
block drop out proto tcp from any to any port 9010  
block drop out proto tcp from any to any port 9011
block drop out proto tcp from any to any port 9012
block drop out proto tcp from any to any port 9013
block drop out proto udp from any to any port 9009
block drop out proto udp from any to any port > 1024
EOF

    echo "✅ Blocked P2P ports 9009-9013 and high UDP ports"
    
elif [[ "$OSTYPE" == "linux-gnu"* ]]; then
    # Linux - Use iptables
    echo "Creating Linux firewall rules..."
    
    # Block croc default ports
    sudo iptables -A OUTPUT -p tcp --dport 9009:9013 -j DROP
    sudo iptables -A OUTPUT -p udp --dport 9009:9013 -j DROP
    sudo iptables -A OUTPUT -p udp --dport 1024:65535 -j DROP
    
    echo "✅ Blocked P2P ports and high UDP ports"
fi

echo ""
echo "🌐 Network restrictions now active:"
echo "   ❌ Croc default ports (9009-9013) blocked"
echo "   ❌ UDP traffic blocked"  
echo "   ❌ P2P protocols blocked"
echo "   ✅ HTTPS (443) still allowed"
echo "   ✅ HTTP (80) still allowed"
echo ""

echo "📋 Test Instructions:"
echo "1. Start TrustDrop on this machine (sender)"
echo "2. Share transfer code with receiver"
echo "3. Monitor which transport is used:"
echo "   - Should fallback to HTTPS tunneling"
echo "   - Should avoid blocked croc ports"
echo "   - Should show 'Restrictive Network Detected'"
echo ""

echo "🔧 To restore normal network access after testing:"
echo "   macOS: sudo pfctl -d"
echo "   Linux: sudo iptables -F"
echo ""

read -p "Press Enter to start TrustDrop with firewall simulation..."
./TrustDrop.app/Contents/MacOS/TrustDrop 