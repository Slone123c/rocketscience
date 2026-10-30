// Official RocketScience component names for each sidebar view.
// Product code refers to these names. Do not invent a second name for the same component.
export const screenComponents = {
  总览: ["FirstRunCard", "IgnitionButton", "TelemetryStat", "SegmentedControl", "Switch", "Button", "Trajectory"],
  代理: ["SegmentedControl", "SearchField", "ProxyGroup"],
  规则: ["SegmentedControl", "Button", "RuleGuide", "OverrideGroup", "SearchField"],
  连接: ["SearchField", "Select", "Button", "ConnectionTable", "ContextMenu", "AddRuleSheet"],
  日志: ["SearchField", "LogStream"],
  配置: ["Button", "ProfileCard", "ProfileEditor"],
  设置: ["SettingsSection", "SettingRow", "SettingField", "Switch", "Select", "TextField", "ListField", "Button"],
  侧栏: ["Sidebar", "CheckPopover", "Icon", "SegmentedControl"],
  引导: ["Tour"],
  菜单栏: ["MenuBarPopover"],
} as const;
