import Foundation
import Security
import SystemConfiguration

let keys = ["HTTPEnable", "HTTPProxy", "HTTPPort", "HTTPSEnable", "HTTPSProxy", "HTTPSPort", "SOCKSEnable", "SOCKSProxy", "SOCKSPort", "ProxyAutoConfigEnable", "ProxyAutoDiscoveryEnable", "ExceptionsList", "__ProtocolEnabled"]
func fail(_ message: String) -> Never { FileHandle.standardError.write(Data((message + "\n").utf8)); exit(1) }
func jsonOutput(_ value: Any) { do { var data = try JSONSerialization.data(withJSONObject: value, options: [.sortedKeys]); data.append(0x0A); try FileHandle.standardOutput.write(contentsOf: data) } catch { fail("无法编码系统代理状态") } }
func serviceList(_ prefs: SCPreferences, enabledOnly: Bool) -> [SCNetworkService] {
    guard let set = SCNetworkSetCopyCurrent(prefs), let services = SCNetworkSetCopyServices(set) as? [SCNetworkService] else { fail("无法读取当前网络服务") }
    return enabledOnly ? services.filter { SCNetworkServiceGetEnabled($0) } : services
}
func config(_ service: SCNetworkService) -> (SCNetworkProtocol, [String: Any])? {
    guard let proto = SCNetworkServiceCopyProtocol(service, kSCNetworkProtocolTypeProxies) else { return nil }
    let dict = (SCNetworkProtocolGetConfiguration(proto) as? [String: Any]) ?? [:]
    return (proto, dict)
}
func relevant(_ dict: [String: Any]) -> [String: Any] { var result: [String: Any] = [:]; for key in keys { if let value = dict[key] { result[key] = value } }; return result }
func equal(_ a: Any?, _ b: Any?) -> Bool { switch (a,b) { case (nil,nil): return true; case let (a?,b?): return (a as AnyObject).isEqual(b); default: return false } }
func apply(_ changes: [[String: Any]], _ auth: AuthorizationRef) {
    guard let prefs = SCPreferencesCreateWithAuthorization(nil, "RocketScience" as CFString, nil, auth) else { fail("无法打开系统网络配置") }
    guard SCPreferencesLock(prefs, true) else { fail("无法锁定网络设置或授权被拒绝") }
    defer { SCPreferencesUnlock(prefs) }
    let services = serviceList(prefs, enabledOnly: false)
    var conflicts: [String] = []
    var changed = false
    var byService: [String: [[String: Any]]] = [:]
    for change in changes {
        guard let id = change["id"] as? String, change["expected"] is [String: Any], change["desired"] is [String: Any], let changedKeys = change["keys"] as? [String], changedKeys.allSatisfy({ keys.contains($0) }) else { fail("无效的代理变更") }
        byService[id, default: []].append(change)
    }
    for (id, serviceChanges) in byService {
        guard let service = services.first(where: { (SCNetworkServiceGetServiceID($0) as String?) == id }), let (proto, rawCurrent) = config(service) else { conflicts.append(id); continue }
        var current = rawCurrent
        current["__ProtocolEnabled"] = SCNetworkProtocolGetEnabled(proto)
        var next = current
        var serviceChanged = false
        for change in serviceChanges {
            let expected = change["expected"] as! [String: Any]
            let desired = change["desired"] as! [String: Any]
            let changedKeys = change["keys"] as! [String]
            let guardKeys = (change["guardKeys"] as? [String]) ?? changedKeys
            guard guardKeys.allSatisfy({ keys.contains($0) }) else { fail("无效的代理校验字段") }
            if !guardKeys.allSatisfy({ equal(current[$0], expected[$0]) }) { conflicts.append(id); continue }
            for key in changedKeys { if let value = desired[key] { next[key] = value } else { next.removeValue(forKey: key) } }
            serviceChanged = true
        }
        if serviceChanged {
            guard let enabled = next.removeValue(forKey: "__ProtocolEnabled") as? Bool else { fail("缺少代理协议状态") }
            guard SCNetworkProtocolSetConfiguration(proto, next as CFDictionary) else { fail("无法写入网络代理配置") }
            guard SCNetworkProtocolSetEnabled(proto, enabled) else { fail("无法设置代理协议状态") }
            changed = true
        }
    }
    if changed {
        guard SCPreferencesCommitChanges(prefs) else { fail("无法保存网络代理配置") }
        guard SCPreferencesApplyChanges(prefs) else { fail("网络配置已保存，但应用失败；请恢复网络") }
    }
    jsonOutput(["conflicts": conflicts])
    fflush(stdout)
}
let mode = CommandLine.arguments.dropFirst().first ?? ""
if mode == "snapshot" {
    guard let prefs = SCPreferencesCreate(nil, "RocketScience" as CFString, nil) else { fail("无法打开系统网络配置") }
    var list: [[String: Any]] = []
    for service in serviceList(prefs, enabledOnly: true) {
        guard let id = SCNetworkServiceGetServiceID(service) as String?, !id.isEmpty, let (proto, dict) = config(service) else { continue }
        var state = relevant(dict)
        state["__ProtocolEnabled"] = SCNetworkProtocolGetEnabled(proto)
        list.append(["id": id, "name": (SCNetworkServiceGetName(service) as String?) ?? "", "settings": state])
    }
    jsonOutput(list)
} else if mode == "cas" {
    // One process serves every write until the app closes it, so the
    // authorization granted for the first change covers restore on stop.
    var auth: AuthorizationRef?
    guard AuthorizationCreate(nil, nil, [], &auth) == errAuthorizationSuccess, let auth else { fail("无法请求网络设置授权") }
    defer { AuthorizationFree(auth, []) }

    while true {
        guard let line = readLine(strippingNewline: true) else { break }
        if line.isEmpty { jsonOutput(["conflicts": [String]()]); continue }
        guard let changes = (try? JSONSerialization.jsonObject(with: Data(line.utf8))) as? [[String: Any]] else { fail("无效的代理请求") }
        apply(changes, auth)
    }
} else { fail("无效的系统代理操作") }


