import React, { useEffect, useState } from "react";
import { createPortal } from "react-dom";

export function PageTools({ children }: { children: React.ReactNode }) {
  const [host, setHost] = useState<HTMLElement | null>(null);
  useEffect(() => { setHost(document.getElementById("page-tools")); }, []);
  if (!host) return null;
  return createPortal(children, host);
}
