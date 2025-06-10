# 🌍 International Firewall Testing Guide
## Germany ↔ Dallas TX Real-World Simulation

### 🎯 **Test Scenario Setup**

#### **Option A: VPN + Local Firewall (Best Simulation)**
1. **Sender (Germany Simulation):**
   - Use German VPN server (NordVPN, ExpressVPN)
   - Enable strict firewall rules (use test_firewall.sh)
   - Connect through corporate-style proxy

2. **Receiver (Dallas Simulation):**
   - Use Dallas/Texas VPN server
   - Enable Windows Defender Firewall (strict mode)
   - Test from different network (mobile hotspot)

#### **Option B: Public WiFi Testing**
1. **Library/Starbucks WiFi:** Heavy restrictions
2. **University WiFi:** Academic firewalls
3. **Hotel WiFi:** International roaming restrictions
4. **Airport WiFi:** Security-focused blocking

#### **Option C: Real International Testing**
1. **Ask friend in Germany** to test sending
2. **Test from Dallas recipient** 
3. **Use different ISPs** (Verizon, AT&T, etc.)

### 🔬 **What to Monitor During Tests**

#### **Success Indicators:**
- ✅ `Transport Status: HTTPS Tunnel Active`
- ✅ `Network: Restrictive (using fallback)`
- ✅ `Transfer via Port 443`
- ✅ Files transfer despite firewall

#### **Failure Recovery:**
- 🔄 `Croc transport failed, trying HTTPS...`
- 🔄 `Port 9009 blocked, using port 443...`
- 🔄 `P2P blocked, using relay tunnel...`

### 📊 **Expected Results**

| Network Scenario | Primary Transport | Fallback | Success Rate |
|------------------|-------------------|----------|--------------|
| Home → Home | Croc Direct | N/A | 99% |
| Corporate → Home | HTTPS Tunnel | WebSocket | 95% |
| Corporate → Corporate | HTTPS Tunnel | Tor | 90% |
| University → Corporate | HTTPS + Proxy | Multi-hop | 85% |

### 🚨 **Red Flags to Watch For**

❌ **Bad Signs:**
- Transfer fails completely
- No fallback attempted
- "All transports failed" error
- Timeout after 30 seconds

✅ **Good Signs:**
- Multiple transport attempts
- Automatic fallback to HTTPS
- "Restrictive network detected"
- Transfer completes on port 443 