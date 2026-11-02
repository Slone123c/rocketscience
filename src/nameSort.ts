import {useEffect,useState} from "react";
import type {NameSort} from "../design-system/components/index";

const STORAGE_KEY="rs-node-sort";

export function readNameSort(value:string|null):NameSort {
  return value==="asc"||value==="desc"||value==="latency"?value:"none";
}

// One order for every node list: the proxy page and the menu-bar popover share it,
// and a change in one window reaches the other through the storage event.
export function useNameSort():[NameSort,(sort:NameSort)=>void] {
  const [sort,setSort]=useState<NameSort>(()=>{try{return readNameSort(localStorage.getItem(STORAGE_KEY))}catch{return "none"}});
  useEffect(()=>{
    const sync=(event:StorageEvent)=>{if(event.key===STORAGE_KEY)setSort(readNameSort(event.newValue))};
    window.addEventListener("storage",sync);
    return ()=>window.removeEventListener("storage",sync);
  },[]);
  return [sort,(next:NameSort)=>{setSort(next);try{localStorage.setItem(STORAGE_KEY,next)}catch{}}];
}
