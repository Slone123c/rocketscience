import type {ProfileState} from "./mygo";

// Once the backend has committed, a failed read must not turn the operation
// into a failed save (or invite the editor to submit that same change again).
export async function commitProfileChange(
  write:()=>Promise<ProfileState>,
  accept:(state:ProfileState)=>void,
  refresh:()=>Promise<void>,
  warn:(message:string)=>void,
  completed:string
) {
  const next=await write();
  accept(next);
  try{await refresh()}catch(error){warn(`${completed}，但界面刷新失败：${String(error)}`)}
  return next;
}

// A failed update can still persist lastAttempt/lastError on the backend.
// Reload those fields when possible, retaining both errors when that read fails.
export async function subscriptionFailureMessage(error:unknown,refresh:()=>Promise<void>) {
  try{await refresh();return String(error)}
  catch(readError){return `${String(error)}；无法读取更新状态：${String(readError)}`}
}
