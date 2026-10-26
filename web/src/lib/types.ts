// What the PC sends. Field names follow the Go server's JSON.

export interface Control {
  id: string;
  cid?: string;
  label: string;
  icon?: string;
  keys?: string;
  mode?: string; // "hold" | "timed" | "" (tap)
  state?: { group: string };
  locked?: boolean;
  danger?: boolean;
  unbound?: boolean;
  global?: boolean;
}

export interface Group {
  id: string;
  name: string;
  code?: string;
  category?: string;
  layout?: any;
  size?: { weight?: number; min?: number };
  controls: Control[];
}

export interface View {
  id: string;
  name: string;
  groups: string[];
  preset?: boolean;
}

export interface ConsoleInfo {
  id: string;
  name: string;
  order: number;
  online: boolean;
  panels: number;
  hostedBy?: string;
}

export interface Cockpit {
  me: string;
  consoles: ConsoleInfo[];
  theme?: string;
}

export interface ChatLine {
  who: "you" | "agent" | "system";
  text: string;
  at: string;
}

export interface Battle {
  ship: string;
  fire: boolean;
  from: string;
  hits: number;
  level: number;
  dead: boolean;
  kills: number;
}

export interface Request {
  id: string;
  from: string;
  label?: string;
  text?: string;
}

export interface Crewmate {
  id: string;
  name: string;
  online: boolean;
  afk?: boolean;
  guest?: boolean;
  peer?: boolean;
  afkOnly?: boolean;
  buttons?: { id: string; label: string }[];
}
