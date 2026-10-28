export const settingsTopics = [
  {id:"proxy",label:"代理与共享",group:"basic"},
  {id:"general",label:"通用",group:"basic"},
  {id:"dns",label:"DNS",group:"advanced"},
  {id:"net",label:"端口与连接",group:"advanced"},
  {id:"core",label:"内核",group:"advanced"}
] as const;
export type SettingsTopic = typeof settingsTopics[number]["id"];
export function settingsTopic(value:unknown):SettingsTopic {
  return settingsTopics.some(topic=>topic.id===value)?value as SettingsTopic:"proxy";
}
export function navigateSettings(current:SettingsTopic,page:string,target?:string):{page:string;topic:SettingsTopic} {
  return {page,topic:page==="设置"&&target!==undefined?settingsTopic(target):current};
}
