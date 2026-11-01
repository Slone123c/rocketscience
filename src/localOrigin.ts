import type {IPLocation} from "./mygo";

export interface LocalOrigin {
  /** [lon, lat], or null when nothing says where this Mac is. */
  geo:[number,number]|null;
  source:"ip"|"timezone"|null;
  /** Shown under the map: where the origin came from, or why it is missing. */
  note:string;
}

// Time zones mapped to their city, for when the IP lookup fails.
const ZONES:Record<string,[number,number]>={
  "Asia/Shanghai":[121.5,31.2],"Asia/Chongqing":[106.5,29.6],"Asia/Harbin":[126.6,45.8],"Asia/Urumqi":[87.6,43.8],
  "Asia/Hong_Kong":[114.2,22.3],"Asia/Macau":[113.5,22.2],"Asia/Taipei":[121.5,25],"Asia/Tokyo":[139.7,35.7],
  "Asia/Seoul":[127,37.6],"Asia/Singapore":[103.8,1.35],"Asia/Kuala_Lumpur":[101.7,3.1],"Asia/Bangkok":[100.5,13.75],
  "Asia/Ho_Chi_Minh":[106.7,10.8],"Asia/Manila":[121,14.6],"Asia/Jakarta":[106.8,-6.2],"Asia/Kolkata":[72.9,19.1],
  "Asia/Dubai":[55.3,25.2],"Europe/Istanbul":[29,41],"Europe/Moscow":[37.6,55.75],"Europe/London":[-0.1,51.5],
  "Europe/Paris":[2.35,48.85],"Europe/Berlin":[13.4,52.5],"Europe/Amsterdam":[4.9,52.4],"Australia/Sydney":[151.2,-33.9],
  "Australia/Melbourne":[145,-37.8],"America/Los_Angeles":[-118.2,34.1],"America/Denver":[-105,39.7],
  "America/Chicago":[-87.6,41.9],"America/New_York":[-74,40.7],"America/Toronto":[-79.4,43.7],
  "America/Vancouver":[-123.1,49.3],"America/Sao_Paulo":[-46.6,-23.5],"America/Argentina/Buenos_Aires":[-58.4,-34.6]
};

/**
 * The public IP's city first (looked up directly, not through the proxy), then
 * the system time zone. Neither needs a permission prompt. Never a made-up
 * default: an unknown zone leaves the origin out.
 */
export function localOrigin(ip:IPLocation|null,timeZone:string,geoOf:(name:string)=>[number,number]|null):LocalOrigin {
  if(ip?.known){
    const place=[ip.city,ip.region,ip.country].filter((part,i,all)=>part&&all.indexOf(part)===i).join("，");
    return {geo:[ip.lon,ip.lat],source:"ip",note:`本机位置按公网 IP 推测：${place||"未知城市"}（${ip.source}）`};
  }
  const city=timeZone.split("/").pop()?.replace(/_/g," ")||"";
  const zone=ZONES[timeZone]??(city?geoOf(city):null);
  const why=ip?"公网 IP 查询失败":"正在查询公网 IP";
  if(zone)return {geo:zone,source:"timezone",note:`本机位置按时区 ${timeZone} 推测（${why}）`};
  return {geo:null,source:null,note:`本机位置未知（${why}，时区 ${timeZone||"未知"} 无法对应城市）`};
}
