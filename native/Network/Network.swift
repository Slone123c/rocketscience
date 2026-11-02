import Foundation
import AppKit
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

// These exports run on Go threads, which never drain an autorelease pool, so each call brings its own.
@_cdecl("rs_wifi_snapshot")
public func wifiSnapshot() -> UnsafeMutablePointer<CChar>? {
    autoreleasepool { encodedWiFiSnapshot() }
}

private func encodedWiFiSnapshot() -> UnsafeMutablePointer<CChar>? {
    guard let data = try? JSONSerialization.data(withJSONObject: snapshot()), let json = String(data: data, encoding: .utf8) else { return nil }
    return strdup(json)
}

@_cdecl("rs_wifi_free")
public func freeWiFiSnapshot(_ pointer: UnsafeMutablePointer<CChar>?) { free(pointer) }

// The outermost .app around an executable, so helpers (Chrome's "Helper (Renderer).app") show their parent app.
private func enclosingApp(_ path: String) -> String? {
    var url = URL(fileURLWithPath: path)
    var found: String?
    while url.path != "/" {
        if url.pathExtension == "app" { found = url.path }
        url.deleteLastPathComponent()
    }
    return found
}

// Returns {"name","icon"} as JSON: the app's display name and a 64 px PNG data URI. Command-line tools return null.
@_cdecl("rs_app_icon")
public func appIcon(_ path: UnsafePointer<CChar>?) -> UnsafeMutablePointer<CChar>? {
    autoreleasepool { encodedAppIcon(path) }
}

private func encodedAppIcon(_ path: UnsafePointer<CChar>?) -> UnsafeMutablePointer<CChar>? {
    guard let path, let app = enclosingApp(String(cString: path)) else { return nil }
    let image = NSWorkspace.shared.icon(forFile: app)
    let side = 64
    guard let rep = NSBitmapImageRep(bitmapDataPlanes: nil, pixelsWide: side, pixelsHigh: side, bitsPerSample: 8, samplesPerPixel: 4, hasAlpha: true, isPlanar: false, colorSpaceName: .deviceRGB, bytesPerRow: 0, bitsPerPixel: 0) else { return nil }
    NSGraphicsContext.saveGraphicsState()
    NSGraphicsContext.current = NSGraphicsContext(bitmapImageRep: rep)
    image.draw(in: NSRect(x: 0, y: 0, width: side, height: side), from: .zero, operation: .copy, fraction: 1)
    NSGraphicsContext.restoreGraphicsState()
    guard let png = rep.representation(using: .png, properties: [:]) else { return nil }
    let name = (FileManager.default.displayName(atPath: app) as NSString).deletingPathExtension
    let value: [String: Any] = ["name": name, "icon": "data:image/png;base64," + png.base64EncodedString()]
    guard let data = try? JSONSerialization.data(withJSONObject: value), let json = String(data: data, encoding: .utf8) else { return nil }
    return strdup(json)
}

// The menu-bar speed readout: two stacked lines, ↑ over ↓, drawn black on clear as a template image
// 36 px tall (18 pt at 2x). Width is fixed by the widest possible value so the status item never jitters.
@_cdecl("rs_tray_speeds")
public func traySpeeds(_ up: UnsafePointer<CChar>?, _ down: UnsafePointer<CChar>?) -> UnsafeMutablePointer<CChar>? {
    autoreleasepool { encodedTraySpeeds(up, down) }
}

private func encodedTraySpeeds(_ up: UnsafePointer<CChar>?, _ down: UnsafePointer<CChar>?) -> UnsafeMutablePointer<CChar>? {
    guard let up, let down else { return nil }
    let font = NSFont.monospacedDigitSystemFont(ofSize: 17, weight: .medium)
    let ink: [NSAttributedString.Key: Any] = [.font: font, .foregroundColor: NSColor.black]
    let dim: [NSAttributedString.Key: Any] = [.font: NSFont.systemFont(ofSize: 13, weight: .semibold), .foregroundColor: NSColor.black.withAlphaComponent(0.55)]
    let arrow = max(("↑" as NSString).size(withAttributes: dim).width, ("↓" as NSString).size(withAttributes: dim).width)
    let number = ["888B", "8.8K", "888K", "8.8M", "888M", "8.8G"].map { ($0 as NSString).size(withAttributes: ink).width }.max() ?? 0
    let gap: CGFloat = 2
    let width = Int(ceil(arrow + gap + number))
    let height = 36
    guard let rep = NSBitmapImageRep(bitmapDataPlanes: nil, pixelsWide: width, pixelsHigh: height, bitsPerSample: 8, samplesPerPixel: 4, hasAlpha: true, isPlanar: false, colorSpaceName: .deviceRGB, bytesPerRow: 0, bitsPerPixel: 0) else { return nil }
    NSGraphicsContext.saveGraphicsState()
    NSGraphicsContext.current = NSGraphicsContext(bitmapImageRep: rep)
    for (row, glyph, value) in [(1, "↑", String(cString: up)), (0, "↓", String(cString: down))] {
        // Each line owns half the height; draw(at:) places the line box's bottom-left corner.
        let half = CGFloat(height / 2), base = CGFloat(row) * half
        let arrowSize = (glyph as NSString).size(withAttributes: dim)
        (glyph as NSString).draw(at: NSPoint(x: 0, y: base + (half - arrowSize.height) / 2), withAttributes: dim)
        let size = (value as NSString).size(withAttributes: ink)
        (value as NSString).draw(at: NSPoint(x: CGFloat(width) - size.width, y: base + (half - size.height) / 2), withAttributes: ink)
    }
    NSGraphicsContext.restoreGraphicsState()
    guard let png = rep.representation(using: .png, properties: [:]) else { return nil }
    let value: [String: Any] = ["png": png.base64EncodedString(), "width": width]
    guard let data = try? JSONSerialization.data(withJSONObject: value), let json = String(data: data, encoding: .utf8) else { return nil }
    return strdup(json)
}
