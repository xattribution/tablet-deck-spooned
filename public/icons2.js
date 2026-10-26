// More icons for the catalog's controls, and which icon and category each control
// gets. Categories are colour-coded the same on every theme, so flight, engineering,
// combat and the rest read at a glance.
(() => {
  const I = window.DECK_ICONS;
  Object.assign(I, {
    shield: `<path d="M24 5 39 11v11c0 10-6.5 17-15 21C15.5 39 9 32 9 22V11Z"/>`,
    "shield-front": `<path d="M24 9 37 14v9c0 8-5.5 14-13 17-7.5-3-13-9-13-17v-9Z"/><path d="M15 6h18" stroke-width="4"/>`,
    "shield-back": `<path d="M24 5 37 10v9c0 8-5.5 14-13 17-7.5-3-13-9-13-17v-9Z"/><path d="M15 43h18" stroke-width="4"/>`,
    "shield-left": `<path d="M27 6 40 11v9c0 8-5.5 14-13 17-7.5-3-13-9-13-17v-9Z"/><path d="M6 14v18" stroke-width="4"/>`,
    "shield-right": `<path d="M21 6 34 11v9c0 8-5.5 14-13 17C13.5 34 8 28 8 20v-9Z"/><path d="M42 14v18" stroke-width="4"/>`,
    "shield-top": `<path d="M24 12 37 17v8c0 7-5.5 12-13 15-7.5-3-13-8-13-15v-8Z"/><path d="M12 6h24" stroke-width="4"/><path d="M24 6v4"/>`,
    "shield-bottom": `<path d="M24 4 37 9v8c0 7-5.5 12-13 15-7.5-3-13-8-13-15V9Z"/><path d="M12 42h24" stroke-width="4"/><path d="M24 36v4"/>`,
    "shield-reset": `<path d="M24 5 39 11v11c0 10-6.5 17-15 21C15.5 39 9 32 9 22V11Z"/><path d="M17 24a7 7 0 1 0 2-5m-2-4v4h4"/>`,
    triangle: `<path d="M24 7 42 39H6Z"/><circle cx="24" cy="29" r="3.5" fill="currentColor" stroke="none"/>`,
    esp: `<circle cx="24" cy="24" r="16"/><path d="M24 8v6M24 34v6M8 24h6M34 24h6"/><path d="m18 30 12-12" stroke-width="3"/>`,
    gforce: `<path d="M8 36c8-16 24-16 32 0"/><text x="24" y="31" text-anchor="middle" font-size="15" font-weight="700" fill="currentColor" stroke="none" font-family="sans-serif">G</text>`,
    warning: `<path d="M24 6 43 40H5Z"/><path d="M24 18v11"/><circle cx="24" cy="34" r="1.8" fill="currentColor" stroke="none"/>`,
    autoland: `<ellipse cx="24" cy="38" rx="17" ry="5"/><path d="M24 6v20m-7-7 7 7 7-7"/><path d="M14 10h4M30 10h4"/>`,
    docking: `<rect x="14" y="14" width="20" height="20"/><path d="M24 4v8M24 36v8M4 24h8M36 24h8"/>`,
    beam: `<path d="M6 40 22 24M18 40 30 28M30 40 38 32"/><circle cx="34" cy="12" r="6"/>`,
    modes: `<rect x="7" y="9" width="34" height="30"/><path d="M7 19h34M18 9v10" /><path d="M14 29h20" opacity=".6"/>`,
    triangleup: `<path d="M24 7 42 39H6Z"/>`,
  });

  // Category of each group, and the order categories are listed in.
  const CAT = {
    power: "eng", systems: "eng", flight: "flight", landing: "flight", quantum: "nav", interface: "ui", modes: "ui",
    shields: "def", targeting: "combat", combat: "combat", emergency: "emerg", mining: "ind", salvage: "ind",
    crew: "crew", "on-foot": "crew", emotes: "crew",
  };
  const CAT_NAMES = { flight: "Flight", eng: "Engineering", combat: "Combat", def: "Defense", nav: "Navigation", ind: "Industry", crew: "Crew", ui: "Interface", emerg: "Emergency" };

  // Icon by control id; the first rule that matches wins, after a layout's own icon.
  const RULES = [
    [/landing-gear/, "gear"], [/vtol/, "vtol"], [/request-(landing|docking)|^atc$/, "request-landing"], [/auto-land/, "autoland"],
    [/docking-camera/, "docking"], [/flight-ready/, "flight-ready"], [/power-on|^pwr$/, "power"],
    [/^wep$|weapons/, "power-weapons"], [/^thr$|thrusters|engines/, "power-engines"], [/^shld$|power-shields|max-shields|min-shields|^shields-(up|down)$/, "power-shields"],
    [/reset-all|power-reset/, "power-reset"],
    [/shields-front/, "shield-front"], [/shields-back/, "shield-back"], [/shields-left/, "shield-left"], [/shields-right/, "shield-right"],
    [/shields-top/, "shield-top"], [/shields-bottom/, "shield-bottom"], [/shields-reset/, "shield-reset"],
    [/esp/, "esp"], [/g-(force-)?safety|gsafe/, "gforce"], [/look-behind|inspect/, "eye"], [/match/, "target"], [/speed|limit/, "speed"],
    [/master-mode|scm-nav/, "swap"], [/flight-mode|^flight$/, "ship"], [/guns/, "target"], [/missile/, "missile"],
    [/quantum-mode|^quantum$/, "quantum-mode"], [/quantum/, "quantum-jump"], [/scan/, "scanning"],
    [/mining-mode|^mining$/, "mining"], [/fire-laser|fracture/, "laser"], [/laser-power-up|beam-spacing-up/, "chev-up"],
    [/laser-power-down|beam-spacing-down/, "chev-down"], [/module/, "module"], [/salvage/, "salvage"], [/modifier/, "swap"],
    [/turret/, "turret"], [/unlock/, "unlock"], [/lock/, "lock"], [/doors/, "doors"], [/lights/, "lights"], [/exit-seat|eject/, "exit-seat"],
    [/self-d/, "warning"], [/panic|decoy/, "decoy"], [/noise/, "noise"], [/lead-lag|prev|target/, "target"],
    [/3rd-person|camera/, "camera"], [/walk/, "walk"], [/drop/, "drop"], [/chat/, "chat"], [/contacts/, "party"],
    [/star-map/, "starmap"], [/mobiglas/, "mobiglas"], [/inventory/, "inventory"], [/^esc$/, "escape"],
  ];
  const GROUP_ICON = { power: "triangle", shields: "shield", targeting: "target", combat: "missile", emergency: "warning", emotes: "emote",
    mining: "mining", salvage: "salvage", quantum: "quantum-jump", landing: "gear", flight: "speed", modes: "modes", interface: "mobiglas", crew: "doors", "on-foot": "walk", systems: "power" };

  window.DeckCat = {
    of: (group) => CAT[group] || "ui",
    name: (cat) => CAT_NAMES[cat] || cat,
    icon(c, group) {
      const id = c.id || "";
      if (group === "emotes") return "emote";
      // Our rules first (they keep a system's controls on one icon, told apart by a
      // badge), then the layout's own icon, then the group's.
      for (const [re, name] of RULES) if (re.test(id)) return name;
      if (c.icon && I[c.icon] && c.icon !== "emblem") return c.icon;
      return GROUP_ICON[group] || "default";
    },
    // A short badge for controls that share an icon: + − MAX MIN.
    badge(label) {
      const l = String(label || "");
      if (/^max\b/i.test(l)) return "MAX";
      if (/^min\b/i.test(l)) return "MIN";
      if (/\+\s*$|\bup$/i.test(l)) return "+";
      if (/[−-]\s*$|\bdown$/i.test(l)) return "−";
      return "";
    },
  };
})();
