import Foundation
import CoreWLAN
import CoreLocation
import SystemConfiguration

private final class LocationDelegate: NSObject, CLLocationManagerDelegate {}
private let locationDelegate = LocationDelegate()
private var locationManager: CLLocationManager?

@_cdecl("rs_wifi_request_access")
public func requestWiFiAccess() {
    if locationManager == nil {
        locationManager = CLLocationManager()
        locationManager?.delegate = locationDelegate
    }
    locationManager?.requestWhenInUseAuthorization()
}

private func snapshot() -> [String: Any] {
    guard let interface = CWWiFiClient.shared().interface() else {
        return ["known": true, "ssid": "", "detail": "没有可用的 Wi-Fi 网卡"]
    }
    if !interface.powerOn() {
        return ["known": true, "ssid": "", "detail": "Wi-Fi 已关闭"]
    }
    let primary4 = SCDynamicStoreCopyValue(nil, "State:/Network/Global/IPv4" as CFString) as? [String: Any]
    let primary6 = SCDynamicStoreCopyValue(nil, "State:/Network/Global/IPv6" as CFString) as? [String: Any]
    let primary = (primary4?["PrimaryInterface"] as? String) ?? (primary6?["PrimaryInterface"] as? String)
    if let primary, primary != interface.interfaceName {
        if primary.hasPrefix("utun") || primary.hasPrefix("ppp") || primary.hasPrefix("ipsec") {
            return ["known": false, "ssid": "", "detail": "当前主网络使用虚拟接口，无法确认 Wi-Fi 路由"]
        }
        return ["known": true, "ssid": "", "detail": "当前主网络不是 Wi-Fi"]
    }
    if primary == nil {
        return ["known": false, "ssid": "", "detail": "无法确定当前主网络"]
    }
    guard let ssid = interface.ssid(), !ssid.isEmpty else {
        return ["known": false, "ssid": "", "detail": "无法读取 Wi-Fi 名称；请检查定位权限或网络连接"]
    }
    return ["known": true, "ssid": ssid, "detail": ""]
}

@_cdecl("rs_wifi_snapshot")
public func wifiSnapshot() -> UnsafeMutablePointer<CChar>? {
    guard let data = try? JSONSerialization.data(withJSONObject: snapshot()), let json = String(data: data, encoding: .utf8) else { return nil }
    return strdup(json)
}

@_cdecl("rs_wifi_free")
public func freeWiFiSnapshot(_ pointer: UnsafeMutablePointer<CChar>?) { free(pointer) }
