// Each visible group owns its own request. Invalidated completions must not
// erase a newer request's indicator, even after returning to the same session.
export function createGroupMeasurements() {
  let session="";
  const pending=new Map<string,object>();
  return {
    setSession(next:string){if(next===session)return false;session=next;pending.clear();return true},
    cancel(){pending.clear()},
    begin(group:string){
      if(!session||pending.has(group))return null;
      const owner={},captured=session;pending.set(group,owner);
      const current=()=>session===captured&&pending.get(group)===owner;
      return {current,finish(){if(!current())return false;pending.delete(group);return true}};
    }
  };
}
