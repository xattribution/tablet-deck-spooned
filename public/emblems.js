// Manufacturer emblems: original badge designs on a 64x64 grid that suggest each maker's
// character; they are not the manufacturers' real logos.
window.DECK_EMBLEMS = {
 "rsi": {
  "name": "RSI",
  "full": "Roberts Space Industries",
  "tag": "Classic interstellar lines",
  "emblem": "<circle cx=\"32\" cy=\"32\" r=\"27\"/><circle cx=\"32\" cy=\"32\" r=\"20\" opacity=\".3\"/> <path d=\"M32 8 40 34 32 28 24 34Z\" fill=\"currentColor\" stroke=\"none\"/> <path d=\"M5 41c12-9 36-17 54-15\" stroke-width=\"3.5\"/> <g style=\"color:var(--text)\"><path d=\"M21 47h22\"/><path d=\"M25 53h14\" opacity=\".6\"/></g>"
 },
 "drake": {
  "name": "Drake",
  "full": "Drake Interplanetary",
  "tag": "Built rough, flown hard",
  "emblem": "<path d=\"M32 4 56 18v28L32 60 8 46V18Z\" stroke-width=\"3.5\"/> <path d=\"M32 12 49 22v20L32 52 15 42V22Z\" opacity=\".3\"/> <path d=\"M19 15c9 8 12 18 10 33l-5-3c1-11-1-20-5-30Z\" fill=\"currentColor\" stroke=\"none\"/> <path d=\"M29 13c9 9 12 19 10 35l-5-3c1-12-1-22-5-32Z\" fill=\"currentColor\" stroke=\"none\"/> <path d=\"M40 17c6 7 9 16 7 27l-4-3c0-9-1-16-3-24Z\" fill=\"currentColor\" stroke=\"none\" opacity=\".75\"/>"
 },
 "aegis": {
  "name": "Aegis",
  "full": "Aegis Dynamics",
  "tag": "Military precision",
  "emblem": "<path d=\"M27 24 4 40h23Z\" fill=\"currentColor\" stroke=\"none\" opacity=\".55\"/><path d=\"M37 24l23 16H37Z\" fill=\"currentColor\" stroke=\"none\" opacity=\".55\"/> <path d=\"M27 31 12 44h15M37 31l15 13H37\"/> <path d=\"M32 3 38 30 32 54 26 30Z\" fill=\"currentColor\" stroke=\"none\"/> <g style=\"color:var(--text)\"><path d=\"M21 50h22M27 59h10\"/></g>"
 },
 "anvil": {
  "name": "Anvil",
  "full": "Anvil Aerospace",
  "tag": "Frontline hardware",
  "emblem": "<path d=\"M32 4 56 12v18c0 14-10 24-24 30C18 54 8 44 8 30V12Z\" stroke-width=\"3.5\"/> <path d=\"M32 10 50 16v14c0 11-8 19-18 24-10-5-18-13-18-24V16Z\" opacity=\".3\"/> <path d=\"M15 17h34v6H15Z\" fill=\"currentColor\" stroke=\"none\"/> <g style=\"color:var(--text)\"><path d=\"M20 30l12 8 12-8M20 40l12 8 12-8\" stroke-width=\"3.5\"/></g>"
 },
 "origin": {
  "name": "Origin",
  "full": "Origin Jumpworks",
  "tag": "Luxury in the void",
  "emblem": "<circle cx=\"32\" cy=\"32\" r=\"26\" stroke-width=\"1.6\"/> <path d=\"M40 10a22 22 0 1 0 0 44a30 30 0 0 1 0-44Z\" fill=\"currentColor\" stroke=\"none\"/> <circle cx=\"45\" cy=\"21\" r=\"2.2\" fill=\"currentColor\" stroke=\"none\"/> <g style=\"color:var(--text)\"><path d=\"M32 1v9M32 54v9\" stroke-width=\"1.6\"/></g>"
 },
 "misc": {
  "name": "MISC",
  "full": "Musashi Industrial & Starflight",
  "tag": "Industrial workhorses",
  "emblem": "<path d=\"M18 8h28l14 24-14 24H18L4 32Z\" stroke-width=\"3\"/> <rect x=\"19\" y=\"19\" width=\"26\" height=\"6\" rx=\"1\" fill=\"currentColor\" stroke=\"none\"/> <rect x=\"14\" y=\"29\" width=\"36\" height=\"6\" rx=\"1\" fill=\"currentColor\" stroke=\"none\"/> <rect x=\"19\" y=\"39\" width=\"26\" height=\"6\" rx=\"1\" fill=\"currentColor\" stroke=\"none\" opacity=\".6\"/> <g style=\"color:var(--text)\"><path d=\"M8 32h3M53 32h3\"/></g>"
 },
 "crusader": {
  "name": "Crusader",
  "full": "Crusader Industries",
  "tag": "Open skies, big haulers",
  "emblem": "<circle cx=\"32\" cy=\"32\" r=\"27\"/> <path d=\"M19 46C23 28 35 16 51 12c-6 10-9 22-9 34Z\" fill=\"currentColor\" stroke=\"none\"/> <path d=\"M13 46c3-10 8-17 15-22-2 7-3 14-3 22Z\" fill=\"currentColor\" stroke=\"none\" opacity=\".45\"/> <g style=\"color:var(--text)\"><path d=\"M8 46h48\"/><path d=\"M15 52h34\" opacity=\".4\"/></g>"
 },
 "banu": {
  "name": "Banu",
  "full": "Banu Souli",
  "tag": "Alien craft, merchant spirit",
  "emblem": "<circle cx=\"32\" cy=\"32\" r=\"28\" stroke-dasharray=\"1 5\"/> <path d=\"M32 5c7 8 7 15 0 21-7-6-7-13 0-21Z\" fill=\"currentColor\" stroke=\"none\" transform=\"rotate(0 32 32)\"/><path d=\"M32 5c7 8 7 15 0 21-7-6-7-13 0-21Z\" fill=\"currentColor\" stroke=\"none\" transform=\"rotate(90 32 32)\"/><path d=\"M32 5c7 8 7 15 0 21-7-6-7-13 0-21Z\" fill=\"currentColor\" stroke=\"none\" transform=\"rotate(180 32 32)\"/><path d=\"M32 5c7 8 7 15 0 21-7-6-7-13 0-21Z\" fill=\"currentColor\" stroke=\"none\" transform=\"rotate(270 32 32)\"/> <path d=\"M32 12c4 5 4 9 0 13-4-4-4-8 0-13Z\" fill=\"currentColor\" stroke=\"none\" opacity=\".5\" transform=\"rotate(45 32 32)\"/><path d=\"M32 12c4 5 4 9 0 13-4-4-4-8 0-13Z\" fill=\"currentColor\" stroke=\"none\" opacity=\".5\" transform=\"rotate(135 32 32)\"/><path d=\"M32 12c4 5 4 9 0 13-4-4-4-8 0-13Z\" fill=\"currentColor\" stroke=\"none\" opacity=\".5\" transform=\"rotate(225 32 32)\"/><path d=\"M32 12c4 5 4 9 0 13-4-4-4-8 0-13Z\" fill=\"currentColor\" stroke=\"none\" opacity=\".5\" transform=\"rotate(315 32 32)\"/> <g style=\"color:var(--text)\"><circle cx=\"32\" cy=\"32\" r=\"4.5\"/></g>"
 },
 "aopoa": {
  "name": "Aopoa",
  "full": "Aopoa",
  "tag": "Xi'an elegance",
  "emblem": "<path d=\"M12 52C9 31 22 13 45 8 32 18 27 31 29 52Z\" fill=\"currentColor\" stroke=\"none\"/> <path d=\"M52 12c3 21-10 38-33 44 13-10 18-24 16-44Z\" fill=\"currentColor\" stroke=\"none\" opacity=\".5\"/> <g style=\"color:var(--text)\"><circle cx=\"32\" cy=\"32\" r=\"2.6\" fill=\"currentColor\" stroke=\"none\"/></g>"
 },
 "argo": {
  "name": "Argo",
  "full": "Argo Astronautics",
  "tag": "Utility first",
  "emblem": "<path d=\"M32 5 59 55H5Z\" stroke-width=\"3.5\"/><path d=\"M32 19 47 47H17Z\" opacity=\".3\"/> <circle cx=\"14\" cy=\"50\" r=\"2.4\" fill=\"currentColor\" stroke=\"none\"/><circle cx=\"50\" cy=\"50\" r=\"2.4\" fill=\"currentColor\" stroke=\"none\"/> <g style=\"color:var(--text)\"><path d=\"M32 15v15a5 5 0 1 1-5 5\" stroke-width=\"3.5\"/></g>"
 },
 "cnou": {
  "name": "CO",
  "full": "Consolidated Outland",
  "tag": "Frontier newcomer",
  "emblem": "<path d=\"M48 14a24 24 0 1 0 0 36\" stroke-width=\"6\"/> <path d=\"M33 41a9 9 0 0 1 18 0Z\" fill=\"currentColor\" stroke=\"none\"/> <g style=\"color:var(--text)\"><path d=\"M29 41h30\"/><path d=\"M42 26v-4M32 30l-3-3M52 30l3-3\" opacity=\".7\"/></g>"
 },
 "esperia": {
  "name": "Esperia",
  "full": "Esperia",
  "tag": "Reborn Tevarin designs",
  "emblem": "<path d=\"M8 32A24 24 0 0 1 32 8\" opacity=\".4\"/> <path d=\"M10 52C14 27 30 10 55 8 39 16 29 30 27 54Z\" fill=\"currentColor\" stroke=\"none\"/> <g style=\"color:var(--text)\"><path d=\"M45 30l2.5 8 8 2.5-8 2.5-2.5 8-2.5-8-8-2.5 8-2.5Z\" fill=\"currentColor\" stroke=\"none\"/></g>"
 }
};
