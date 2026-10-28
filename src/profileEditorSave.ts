import type {Profile,ProfileState,SubscriptionDownload} from "./mygo";
import type {Profile as EditorProfile} from "../design-system/components/index";

interface ProfileWriter {fetch:(url:string)=>Promise<SubscriptionDownload>;save:(profile:Profile,expected:string)=>Promise<ProfileState>}
const zeroTime="0001-01-01T00:00:00Z";
export async function saveEditorProfile(draft:EditorProfile,original:Profile|null,writer:ProfileWriter):Promise<ProfileState> {
  const profile:Profile={id:original?.id||"",name:draft.name.trim(),source:draft.source,url:draft.source==="url"?(draft.url||"").trim():"",interval:draft.source==="url"?Number(draft.interval||"24"):0,content:draft.content,path:draft.source==="file"?draft.path||original?.path||"":"",updatedAt:original?.updatedAt||zeroTime,lastAttempt:original?.lastAttempt||zeroTime,usage:draft.source==="url"?original?.usage||null:null,lastError:""};
  if(profile.source==="url"&&(!original||original.url!==profile.url||!original.content)) {
    const downloaded=await writer.fetch(profile.url);
    profile.content=downloaded.content;profile.usage=downloaded.usage;profile.updatedAt=downloaded.updatedAt;profile.lastAttempt=downloaded.updatedAt;
  }
  return writer.save(profile,original?.updatedAt||"");
}
