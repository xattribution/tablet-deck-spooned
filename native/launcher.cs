// SpoolUp's app window. It starts TabletDeckServer.exe (server.js packed by
// scripts/build.js) hidden, with --ui, and shows its address, pairing code, QR code,
// status and log instead of a console window. Minimizing hides it to a tray icon; closing
// it stops the server, which releases any held keys first.
//
//   TabletDeck.exe [--minimized] [server options such as --dry-run or --port 8791]
//
// The server reports status as "@ui {json}" lines on stdout (see ui() in server.js); every
// other line goes to the log. "quit" on its stdin stops it. Only the installed app uses
// this window; running from source keeps the console.
//
// Updates: unless turned off, the window asks GitHub for the latest release when it starts
// and once a day. A newer version shows a banner; "Update now" downloads the installer,
// checks it against the release's SHA-256 file and runs it quietly. The installer asks this
// window to quit (TabletDeck.Quit), installs, and starts SpoolUp again.
using System;
using System.Collections.Generic;
using System.ComponentModel;
using System.Diagnostics;
using System.Drawing;
using System.Drawing.Drawing2D;
using System.Drawing.Imaging;
using System.Drawing.Text;
using System.Globalization;
using System.IO;
using System.Net;
using System.Runtime.InteropServices;
using System.Security.Cryptography;
using System.Text;
using System.Threading;
using System.Web.Script.Serialization;
using System.Windows.Forms;
using Microsoft.Win32;

static class Program
{
    [DllImport("user32.dll")]
    static extern bool SetProcessDPIAware();

    [DllImport("user32.dll", CharSet = CharSet.Unicode)]
    static extern int RegisterWindowMessage(string name);

    [DllImport("user32.dll")]
    static extern bool PostMessage(IntPtr hWnd, int msg, IntPtr wParam, IntPtr lParam);

    // Sent to every window when SpoolUp is opened a second time; the running copy shows itself.
    public static readonly int ShowWindowMessage = RegisterWindowMessage("TabletDeck.ShowWindow");
    // Sent by the installer (installer/TabletDeck.iss) before it updates or removes the files.
    public static readonly int QuitMessage = RegisterWindowMessage("TabletDeck.Quit");

    [STAThread]
    static void Main(string[] args)
    {
        bool first;
        using (var mutex = new Mutex(true, "Local\\TabletDeck.Window", out first))
        {
            if (!first)
            {
                PostMessage((IntPtr)0xffff, ShowWindowMessage, IntPtr.Zero, IntPtr.Zero); // HWND_BROADCAST
                return;
            }
            SetProcessDPIAware();
            ServicePointManager.SecurityProtocol |= (SecurityProtocolType)3072; // TLS 1.2, which GitHub requires
            Application.SetUnhandledExceptionMode(UnhandledExceptionMode.CatchException);
            Application.ThreadException += (s, e) => Fail(e.Exception);
            AppDomain.CurrentDomain.UnhandledException += (s, e) => Fail(e.ExceptionObject as Exception);
            Application.EnableVisualStyles();
            Application.SetCompatibleTextRenderingDefault(false);
            try { Application.Run(new DeckWindow(args)); }
            catch (Exception ex) { Fail(ex); }
            GC.KeepAlive(mutex);
        }
    }

    // Where Fail saves what went wrong, next to config.json.
    public static readonly string CrashLog = Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.ApplicationData), "TabletDeck", "crash.log");
    static bool failing;

    // Any error nothing else caught: saved to crash.log, shown, and SpoolUp closes. The
    // server notices its window is gone and stops, releasing any held keys.
    static void Fail(Exception ex)
    {
        if (failing) return;
        failing = true;
        string text = ex == null ? "unknown error" : ex.ToString();
        try
        {
            Directory.CreateDirectory(Path.GetDirectoryName(CrashLog));
            File.AppendAllText(CrashLog, DateTime.Now.ToString("yyyy-MM-dd HH:mm:ss", CultureInfo.InvariantCulture) + "  SpoolUp " + Application.ProductVersion
                + " on " + Environment.OSVersion + "\r\n" + text + "\r\n\r\n");
        }
        catch (Exception) { }
        try
        {
            MessageBox.Show("SpoolUp ran into a problem and has to close:\n\n" + (ex == null ? text : ex.Message) +
                "\n\nThe details are saved in " + CrashLog + ". Please attach that file to an issue on GitHub " +
                "(github.com/xattribution/SpoolUp/issues).", "SpoolUp", MessageBoxButtons.OK, MessageBoxIcon.Error);
        }
        catch (Exception) { }
        Environment.Exit(1);
    }
}

// The window's languages. Text is written in English and translated where it's shown;
// what the server sends (log lines, voice errors) stays English. The choice is kept in the
// registry; "Automatic" follows Windows' display language. The translations were made with
// AI; corrections from native speakers are welcome.
static class Lang
{
    // The app's old name (Tablet Deck), kept so the language chosen in older versions carries over.
    const string Key = @"Software\Tablet Deck";
    // [code, name] in the order the Settings window lists them; "" is Automatic.
    public static readonly string[][] Choices = {
        new[] { "", "Automatic" }, new[] { "en", "English" }, new[] { "es", "Español" }, new[] { "fr", "Français" }, new[] { "de", "Deutsch" },
    };
    static readonly string[] Columns = { "es", "fr", "de" };
    static int column = -1; // in the tables' rows; -1 is English

    // The saved choice: "", "en", "es", "fr" or "de".
    public static string Saved
    {
        get
        {
            try
            {
                using (var key = Registry.CurrentUser.OpenSubKey(Key))
                    return key == null ? "" : (key.GetValue("Language") as string ?? "");
            }
            catch (Exception) { return ""; }
        }
    }

    static Lang() { Use(Saved); }

    public static void Use(string code)
    {
        if (code == "") code = CultureInfo.CurrentUICulture.TwoLetterISOLanguageName;
        column = Array.IndexOf(Columns, code);
    }

    public static void Save(string code)
    {
        try { using (var key = Registry.CurrentUser.CreateSubKey(Key)) key.SetValue("Language", code); }
        catch (Exception) { }
        Use(code);
    }

    public static string T(string text)
    {
        string[] row;
        return column >= 0 && text != null && table.TryGetValue(text, out row) ? row[column] : text;
    }

    // T with "{0}", "{1}" filled in.
    public static string F(string text, params object[] args)
    {
        return string.Format(T(text), args);
    }

    // English -> [Español, Français, Deutsch].
    static readonly Dictionary<string, string[]> table = new Dictionary<string, string[]>
    {
        // ---- the main window ----
        { "STAR CITIZEN CONTROL DECK", new[] { "PANEL DE CONTROL PARA STAR CITIZEN", "DECK DE COMMANDE POUR STAR CITIZEN", "STEUERDECK FÜR STAR CITIZEN" } },
        { "STARTING", new[] { "INICIANDO", "DÉMARRAGE", "STARTET" } },
        { "ONLINE", new[] { "EN LÍNEA", "EN LIGNE", "ONLINE" } },
        { "TEST MODE", new[] { "MODO PRUEBA", "MODE TEST", "TESTMODUS" } },
        { "STOPPED", new[] { "DETENIDO", "ARRÊTÉ", "GESTOPPT" } },
        { "UPTIME", new[] { "ACTIVO", "DURÉE", "LAUFZEIT" } },
        { "CONNECT", new[] { "CONECTAR", "CONNEXION", "VERBINDEN" } },
        { "LINK", new[] { "ENLACE", "LIEN", "LINK" } },
        { "TELEMETRY", new[] { "TELEMETRÍA", "TÉLÉMÉTRIE", "TELEMETRIE" } },
        { "VOICE", new[] { "VOZ", "VOIX", "STIMME" } },
        { "SYSTEM", new[] { "SISTEMA", "SYSTÈME", "SYSTEM" } },
        { "LOG", new[] { "REGISTRO", "JOURNAL", "PROTOKOLL" } },
        { "NO NETWORK FOUND", new[] { "NO SE ENCONTRÓ RED", "AUCUN RÉSEAU TROUVÉ", "KEIN NETZWERK GEFUNDEN" } },
        { "OFFLINE", new[] { "SIN CONEXIÓN", "HORS LIGNE", "OFFLINE" } },
        { "WAITING FOR THE SERVER", new[] { "ESPERANDO AL SERVIDOR", "EN ATTENTE DU SERVEUR", "WARTE AUF DEN SERVER" } },
        { "SCAN WITH YOUR TABLET'S CAMERA", new[] { "ESCANEA CON TU TABLET", "SCANNEZ AVEC VOTRE TABLETTE", "MIT DER TABLET-KAMERA SCANNEN" } },
        { "ADDRESS", new[] { "DIRECCIÓN", "ADRESSE", "ADRESSE" } },
        { "No network found. Connect this PC to Wi-Fi or a network.", new[] { "No se encontró ninguna red. Conecta este PC a una wifi o a una red.", "Aucun réseau trouvé. Connectez ce PC au Wi-Fi ou à un réseau.", "Kein Netzwerk gefunden. Verbinde diesen PC mit WLAN oder einem Netzwerk." } },
        { "ALSO", new[] { "TAMBIÉN", "AUSSI", "AUCH" } },
        { "PAIRING CODE", new[] { "CÓDIGO DE EMPAREJAMIENTO", "CODE D'APPAIRAGE", "KOPPLUNGSCODE" } },
        { "TABLETS", new[] { "TABLETS", "TABLETTES", "TABLETS" } },
        { "CONNECTED", new[] { "CONECTADAS", "CONNECTÉES", "VERBUNDEN" } },
        { "KEYS GO TO", new[] { "TECLAS A", "TOUCHES VERS", "TASTEN AN" } },
        { "LOG ONLY", new[] { "SOLO REGISTRO", "JOURNAL SEUL", "NUR PROTOKOLL" } },
        { "ANY WINDOW", new[] { "CUALQUIER VENTANA", "TOUTE FENÊTRE", "JEDES FENSTER" } },
        { "TEST MODE: NO KEYS SENT", new[] { "MODO PRUEBA: SIN TECLAS", "MODE TEST : AUCUNE TOUCHE", "TESTMODUS: KEINE TASTEN" } },
        { "ONLY WHILE IT'S IN FRONT", new[] { "SOLO EN PRIMER PLANO", "SI AU PREMIER PLAN", "NUR IM VORDERGRUND" } },
        { "FOCUS CHECK IS OFF", new[] { "SIN CONTROL DE FOCO", "CONTRÔLE DU FOCUS COUPÉ", "FOKUSPRÜFUNG AUS" } },
        { "PRESSES", new[] { "PULSACIONES", "APPUIS", "TASTENDRÜCKE" } },
        { "LAST: {0}", new[] { "ÚLTIMA: {0}", "DERNIER : {0}", "ZULETZT: {0}" } },
        { "NONE YET", new[] { "NINGUNA AÚN", "AUCUN POUR L'INSTANT", "NOCH KEINE" } },
        { "{0} BLOCKED", new[] { "{0} BLOQUEADAS", "{0} BLOQUÉS", "{0} BLOCKIERT" } },
        { "STRICTNESS", new[] { "EXIGENCIA", "EXIGENCE", "STRENGE" } },
        { "Lower accepts more but may mishear. Higher is stricter.", new[] { "Bajo: acepta más, pero se equivoca más. Alto: más estricto.", "Bas : accepte plus, se trompe plus. Haut : plus strict.", "Niedrig: nimmt mehr an, irrt sich eher. Hoch: strenger." } },
        { "Starting...", new[] { "Iniciando...", "Démarrage...", "Startet..." } },
        { "Starting the server...", new[] { "Iniciando el servidor...", "Démarrage du serveur...", "Server wird gestartet..." } },
        { "Running", new[] { "En marcha", "En marche", "Läuft" } },
        { "Running in test mode (dry run): no keys are sent", new[] { "En modo de prueba (dry run): no se envían teclas", "En mode test (dry run) : aucune touche n'est envoyée", "Läuft im Testmodus (Dry Run): Es werden keine Tasten gesendet" } },
        { "Stopped: {0}", new[] { "Detenido: {0}", "Arrêté : {0}", "Gestoppt: {0}" } },
        { "{0} is missing. Reinstall SpoolUp.", new[] { "Falta {0}. Reinstala SpoolUp.", "{0} est introuvable. Réinstallez SpoolUp.", "{0} fehlt. Installiere SpoolUp neu." } },
        { "Couldn't start the server: {0}", new[] { "No se pudo iniciar el servidor: {0}", "Impossible de démarrer le serveur : {0}", "Der Server konnte nicht starten: {0}" } },
        { "The server stopped (exit code {0}). See the log below.", new[] { "El servidor se detuvo (código {0}). Mira el registro de abajo.", "Le serveur s'est arrêté (code {0}). Voir le journal ci-dessous.", "Der Server wurde beendet (Code {0}). Siehe das Protokoll unten." } },
        { "Off", new[] { "Desactivado", "Désactivé", "Aus" } },
        { "Ready: {0} phrases, {1}", new[] { "Listo: {0} frases, {1}", "Prêt : {0} phrases, {1}", "Bereit: {0} Sätze, {1}" } },
        { ", push-to-talk: {0}", new[] { ", pulsar para hablar: {0}", ", appuyer pour parler : {0}", ", Push-to-Talk: {0}" } },
        { "Turned off in settings", new[] { "Desactivado en los ajustes", "Désactivé dans les réglages", "In den Einstellungen ausgeschaltet" } },
        { "Restarting...", new[] { "Reiniciando...", "Redémarrage...", "Startet neu..." } },
        { "Unavailable: {0}", new[] { "No disponible: {0}", "Indisponible : {0}", "Nicht verfügbar: {0}" } },
        { "Last heard: \"{0}\", {1}% sure. Pressed {2}.", new[] { "Último oído: \"{0}\", {1} % seguro. Pulsado: {2}.", "Dernière écoute : \"{0}\", sûr à {1} %. Appuyé : {2}.", "Zuletzt gehört: \"{0}\", {1} % sicher. Gedrückt: {2}." } },
        { "Last heard: \"{0}\", {1}% sure. Below {2}%, so it was ignored.", new[] { "Último oído: \"{0}\", {1} % seguro. Por debajo del {2} %, así que se ignoró.", "Dernière écoute : \"{0}\", sûr à {1} %. Sous {2} %, donc ignoré.", "Zuletzt gehört: \"{0}\", {1} % sicher. Unter {2} %, daher ignoriert." } },
        { "Last heard: something that isn't one of the commands.", new[] { "Último oído: algo que no es uno de los comandos.", "Dernière écoute : quelque chose qui n'est pas une commande.", "Zuletzt gehört: etwas, das kein Befehl ist." } },
        { "SpoolUp (stopped)", new[] { "SpoolUp (detenido)", "SpoolUp (arrêté)", "SpoolUp (gestoppt)" } },
        { " (test mode)", new[] { " (modo prueba)", " (mode test)", " (Testmodus)" } },
        { "Windows treats \"{0}\" as a Public network, so tablets can't connect. In Windows: Settings > Network & Internet > {0} > set it to Private.", new[] { "Windows trata \"{0}\" como red pública, así que las tablets no pueden conectarse. En Windows: Configuración > Red e Internet > {0} > ponla como privada.", "Windows considère \"{0}\" comme un réseau public, donc les tablettes ne peuvent pas se connecter. Dans Windows : Paramètres > Réseau et Internet > {0} > réglez-le sur Privé.", "Windows behandelt \"{0}\" als öffentliches Netzwerk, daher können Tablets sich nicht verbinden. In Windows: Einstellungen > Netzwerk und Internet > {0} > auf Privat stellen." } },
        { "Tablets can't connect", new[] { "Las tablets no pueden conectarse", "Les tablettes ne peuvent pas se connecter", "Tablets können sich nicht verbinden" } },
        { "Windows treats \"{0}\" as a Public network. Set it to Private so tablets on your network can reach SpoolUp.", new[] { "Windows trata \"{0}\" como red pública. Ponla como privada para que las tablets de tu red puedan llegar a SpoolUp.", "Windows considère \"{0}\" comme un réseau public. Passez-le en privé pour que les tablettes de votre réseau puissent joindre SpoolUp.", "Windows behandelt \"{0}\" als öffentliches Netzwerk. Stelle es auf Privat, damit Tablets in deinem Netzwerk SpoolUp erreichen." } },
        { "Choose your network, then Private network. SpoolUp checks again every 20 seconds.", new[] { "Elige tu red y luego Red privada. SpoolUp vuelve a comprobarlo cada 20 segundos.", "Choisissez votre réseau, puis Réseau privé. SpoolUp vérifie à nouveau toutes les 20 secondes.", "Wähle dein Netzwerk, dann Privates Netzwerk. SpoolUp prüft alle 20 Sekunden erneut." } },
        { "Open network settings", new[] { "Abrir configuración de red", "Ouvrir les paramètres réseau", "Netzwerkeinstellungen öffnen" } },
        { "Windows treats \"{0}\" as a Public network. Open SpoolUp to fix it.", new[] { "Windows trata \"{0}\" como red pública. Abre SpoolUp para arreglarlo.", "Windows considère \"{0}\" comme un réseau public. Ouvrez SpoolUp pour corriger cela.", "Windows behandelt \"{0}\" als öffentliches Netzwerk. Öffne SpoolUp, um das zu beheben." } },
        { "Restarting the server for the new settings...", new[] { "Reiniciando el servidor con los nuevos ajustes...", "Redémarrage du serveur avec les nouveaux réglages...", "Server startet mit den neuen Einstellungen neu..." } },
        { "Couldn't save the settings: {0}", new[] { "No se pudieron guardar los ajustes: {0}", "Impossible d'enregistrer les réglages : {0}", "Die Einstellungen konnten nicht gespeichert werden: {0}" } },
        { "SpoolUp isn't running", new[] { "SpoolUp no está en marcha", "SpoolUp n'est pas lancé", "SpoolUp läuft nicht" } },
        { "Start it first; the layout editor is part of the server.", new[] { "Inícialo primero; el editor de diseños forma parte del servidor.", "Lancez-le d'abord ; l'éditeur de disposition fait partie du serveur.", "Starte es zuerst; der Layout-Editor gehört zum Server." } },
        { "SpoolUp is still running", new[] { "SpoolUp sigue en marcha", "SpoolUp tourne toujours", "SpoolUp läuft weiter" } },
        { "Double-click the tray icon to open it, or right-click it to quit.", new[] { "Haz doble clic en el icono de la bandeja para abrirlo, o clic derecho para salir.", "Double-cliquez sur l'icône de la zone de notification pour l'ouvrir, ou faites un clic droit pour quitter.", "Doppelklicke auf das Symbol im Infobereich, um es zu öffnen, oder klicke rechts, um es zu beenden." } },

        // ---- the main window's buttons, switches and links ----
        { "Copy link", new[] { "Copiar enlace", "Copier le lien", "Link kopieren" } },
        { "Copied!", new[] { "¡Copiado!", "Copié !", "Kopiert!" } },
        { "Settings", new[] { "Ajustes", "Réglages", "Einstellungen" } },
        { "Edit layout", new[] { "Editar diseño", "Éditer le deck", "Layout bearbeiten" } },
        { "Restart", new[] { "Reiniciar", "Redémarrer", "Neu starten" } },
        { "Commands", new[] { "Comandos", "Commandes", "Befehle" } },
        { "Start with Windows (in the tray)", new[] { "Iniciar con Windows (en la bandeja)", "Lancer avec Windows (réduit)", "Mit Windows starten (im Infobereich)" } },
        { "Check for updates automatically", new[] { "Buscar actualizaciones a diario", "Chercher les mises à jour", "Automatisch nach Updates suchen" } },
        { "Settings folder", new[] { "Carpeta de ajustes", "Dossier des réglages", "Einstellungsordner" } },
        { "Check for updates now", new[] { "Buscar actualizaciones ahora", "Rechercher les mises à jour", "Jetzt nach Updates suchen" } },
        { "Feedback & ideas", new[] { "Comentarios e ideas", "Avis et idées", "Feedback & Ideen" } },
        { "What's new", new[] { "Novedades", "Nouveautés", "Neuigkeiten" } },
        { "Update now", new[] { "Actualizar ahora", "Mettre à jour", "Jetzt aktualisieren" } },
        { "Later", new[] { "Más tarde", "Plus tard", "Später" } },
        { "Close", new[] { "Cerrar", "Fermer", "Schließen" } },
        { "Show SpoolUp", new[] { "Mostrar SpoolUp", "Afficher SpoolUp", "SpoolUp anzeigen" } },
        { "Quit", new[] { "Salir", "Quitter", "Beenden" } },
        { "Opens the deck in this PC's browser", new[] { "Abre el deck en el navegador de este PC", "Ouvre le deck dans le navigateur de ce PC", "Öffnet das Deck im Browser dieses PCs" } },
        { "Copies the address with the pairing code, to send to a tablet", new[] { "Copia la dirección con el código de emparejamiento, para enviarla a una tablet", "Copie l'adresse avec le code d'appairage, à envoyer à une tablette", "Kopiert die Adresse mit dem Kopplungscode, um sie an ein Tablet zu schicken" } },
        { "Pairing code, port, focus check, tap length, voice language and the Star Citizen keybind profile", new[] { "Código de emparejamiento, puerto, control de foco, duración del toque, idioma de voz y el perfil de teclas de Star Citizen", "Code d'appairage, port, contrôle du focus, durée d'appui, langue vocale et profil de touches Star Citizen", "Kopplungscode, Port, Fokusprüfung, Tippdauer, Sprachbefehl-Sprache und das Star-Citizen-Tastenprofil" } },
        { "Opens the layout editor in your browser: move buttons, change their keys, labels and icons", new[] { "Abre el editor de diseños en tu navegador: mueve botones y cambia sus teclas, nombres e iconos", "Ouvre l'éditeur de disposition dans votre navigateur : déplacez les boutons, changez leurs touches, libellés et icônes", "Öffnet den Layout-Editor im Browser: Tasten verschieben, ihre Tasten, Beschriftungen und Symbole ändern" } },
        { "Everything you can say, and the button each one presses", new[] { "Todo lo que puedes decir y el botón que pulsa cada frase", "Tout ce que vous pouvez dire, et le bouton que chaque phrase active", "Alles, was du sagen kannst, und welche Taste jeder Befehl drückt" } },
        { "Looks for a new version on GitHub when SpoolUp starts and once a day. Click Update now to install it.", new[] { "Busca una versión nueva en GitHub al iniciar SpoolUp y una vez al día. Pulsa Actualizar ahora para instalarla.", "Cherche une nouvelle version sur GitHub au démarrage de SpoolUp et une fois par jour. Cliquez sur Mettre à jour pour l'installer.", "Sucht beim Start von SpoolUp und einmal am Tag auf GitHub nach einer neuen Version. Klicke auf Jetzt aktualisieren, um sie zu installieren." } },
        { "Opens the folder with config.json and your own layouts", new[] { "Abre la carpeta con config.json y tus propios diseños", "Ouvre le dossier contenant config.json et vos propres dispositions", "Öffnet den Ordner mit config.json und deinen eigenen Layouts" } },
        { "Report a problem or suggest an idea on GitHub", new[] { "Informa de un problema o propón una idea en GitHub", "Signalez un problème ou proposez une idée sur GitHub", "Melde ein Problem oder schlag eine Idee auf GitHub vor" } },
        { "Pick your network there and set its network profile to Private", new[] { "Elige ahí tu red y pon su perfil de red como privado", "Choisissez votre réseau et réglez son profil réseau sur Privé", "Wähle dort dein Netzwerk und stelle sein Netzwerkprofil auf Privat" } },

        // ---- updates ----
        { "Couldn't check for updates", new[] { "No se pudo buscar actualizaciones", "Impossible de rechercher les mises à jour", "Konnte nicht nach Updates suchen" } },
        { "GitHub didn't answer.", new[] { "GitHub no respondió.", "GitHub n'a pas répondu.", "GitHub hat nicht geantwortet." } },
        { "SpoolUp is up to date", new[] { "SpoolUp está actualizado", "SpoolUp est à jour", "SpoolUp ist aktuell" } },
        { "You have version {0}, the latest.", new[] { "Tienes la versión {0}, la más reciente.", "Vous avez la version {0}, la plus récente.", "Du hast Version {0}, die neueste." } },
        { "Update not ready yet", new[] { "La actualización aún no está lista", "Mise à jour pas encore prête", "Update noch nicht bereit" } },
        { "Version {0} is out, but its installer isn't attached yet. Try again in a few minutes.", new[] { "La versión {0} ya salió, pero su instalador aún no está adjunto. Inténtalo en unos minutos.", "La version {0} est sortie, mais son installateur n'est pas encore joint. Réessayez dans quelques minutes.", "Version {0} ist erschienen, aber der Installer ist noch nicht angehängt. Versuch es in ein paar Minuten erneut." } },
        { "SpoolUp {0} is available (you have {1}).", new[] { "SpoolUp {0} está disponible (tienes la {1}).", "SpoolUp {0} est disponible (vous avez la {1}).", "SpoolUp {0} ist verfügbar (du hast {1})." } },
        { "SpoolUp {0} is available", new[] { "SpoolUp {0} está disponible", "SpoolUp {0} est disponible", "SpoolUp {0} ist verfügbar" } },
        { "Open SpoolUp and click Update now.", new[] { "Abre SpoolUp y pulsa Actualizar ahora.", "Ouvrez SpoolUp et cliquez sur Mettre à jour.", "Öffne SpoolUp und klicke auf Jetzt aktualisieren." } },
        { "Downloading SpoolUp {0}...", new[] { "Descargando SpoolUp {0}...", "Téléchargement de SpoolUp {0}...", "SpoolUp {0} wird heruntergeladen..." } },
        { "Checking the download...", new[] { "Comprobando la descarga...", "Vérification du téléchargement...", "Download wird geprüft..." } },
        { "Installing SpoolUp {0}. Allow the Windows prompt to continue.", new[] { "Instalando SpoolUp {0}. Acepta el aviso de Windows para continuar.", "Installation de SpoolUp {0}. Autorisez la demande de Windows pour continuer.", "SpoolUp {0} wird installiert. Bestätige die Windows-Abfrage, um fortzufahren." } },
        { "the download failed ({0})", new[] { "la descarga falló ({0})", "le téléchargement a échoué ({0})", "der Download ist fehlgeschlagen ({0})" } },
        { "the download didn't match its checksum, so it wasn't installed", new[] { "la descarga no coincidía con su suma de comprobación, así que no se instaló", "le téléchargement ne correspondait pas à sa somme de contrôle, il n'a donc pas été installé", "der Download passte nicht zu seiner Prüfsumme und wurde daher nicht installiert" } },
        { "couldn't check the download ({0})", new[] { "no se pudo comprobar la descarga ({0})", "impossible de vérifier le téléchargement ({0})", "der Download konnte nicht geprüft werden ({0})" } },
        { "the Windows permission prompt was canceled", new[] { "se canceló el aviso de permisos de Windows", "la demande d'autorisation de Windows a été annulée", "die Windows-Berechtigungsabfrage wurde abgebrochen" } },
        { "the installer finished without updating", new[] { "el instalador terminó sin actualizar", "l'installateur s'est terminé sans mettre à jour", "der Installer wurde ohne Update beendet" } },
        { "the installer stopped (code {0}). Was the Windows prompt canceled?", new[] { "el instalador se detuvo (código {0}). ¿Se canceló el aviso de Windows?", "l'installateur s'est arrêté (code {0}). La demande de Windows a-t-elle été annulée ?", "der Installer wurde beendet (Code {0}). Wurde die Windows-Abfrage abgebrochen?" } },
        { "Update failed: {0}", new[] { "La actualización falló: {0}", "Échec de la mise à jour : {0}", "Update fehlgeschlagen: {0}" } },
        { "Updated to SpoolUp {0}.", new[] { "Actualizado a SpoolUp {0}.", "Mis à jour vers SpoolUp {0}.", "Auf SpoolUp {0} aktualisiert." } },

        // ---- Voice commands window ----
        { "Voice commands", new[] { "Comandos de voz", "Commandes vocales", "Sprachbefehle" } },
        { "Voice commands (not running)", new[] { "Comandos de voz (no activos)", "Commandes vocales (inactives)", "Sprachbefehle (nicht aktiv)" } },
        { "Voice commands ({0})", new[] { "Comandos de voz ({0})", "Commandes vocales ({0})", "Sprachbefehle ({0})" } },
        { "Voice commands ({0} of {1})", new[] { "Comandos de voz ({0} de {1})", "Commandes vocales ({0} sur {1})", "Sprachbefehle ({0} von {1})" } },
        { "COMMANDS", new[] { "COMANDOS", "COMMANDES", "BEFEHLE" } },
        { "Hold the mic button on the tablet and say one of these. Most Star Citizen controls are toggles, so \"engines on\" and \"engines off\" press the same key.", new[] { "Mantén el micro en la tablet y di uno (en inglés). Casi todos los controles alternan: \"engines on\" y \"engines off\" pulsan la misma tecla.", "Maintenez le micro sur la tablette et dites-en un (en anglais). La plupart basculent : \"engines on\" et \"engines off\" appuient sur la même touche.", "Halte das Mikro auf dem Tablet und sag einen davon (Englisch). Die meisten Befehle schalten um: \"engines on\" und \"engines off\" drücken dieselbe Taste." } },
        { "Search, e.g. shields or quantum", new[] { "Buscar, p. ej. shields o quantum", "Rechercher, par ex. shields ou quantum", "Suchen, z. B. shields oder quantum" } },
        { "Say", new[] { "Di", "Dites", "Sag" } },
        { "Presses", new[] { "Pulsa", "Appuie sur", "Drückt" } },

        // ---- Install keybind profile window ----
        { "Install keybind profile", new[] { "Instalar perfil de teclas", "Installer le profil de touches", "Tastenprofil installieren" } },
        { "STAR CITIZEN COPIES", new[] { "COPIAS DE STAR CITIZEN", "COPIES DE STAR CITIZEN", "STAR-CITIZEN-KOPIEN" } },
        { "Copies the SpoolUp Keyboard profile into Star Citizen, so buttons for actions with no default key work. A copy of the game with binds of your own gets \"SpoolUp Keyboard (your binds)\" instead: all of your binds, plus keys only for the deck's actions you haven't bound. Tick the copies to install it into.", new[] { "Copia el perfil SpoolUp Keyboard en Star Citizen, para que funcionen los botones de acciones sin tecla por defecto. Una copia del juego con asignaciones propias recibe \"SpoolUp Keyboard (your binds)\": todas tus asignaciones, más teclas solo para las acciones del panel que no tengas asignadas. Marca las copias donde instalarlo.", "Copie le profil SpoolUp Keyboard dans Star Citizen, pour que les boutons des actions sans touche par défaut fonctionnent. Une copie du jeu avec vos propres raccourcis reçoit plutôt \"SpoolUp Keyboard (your binds)\" : tous vos raccourcis, plus des touches seulement pour les actions du panneau que vous n'avez pas attribuées. Cochez les copies où l'installer.", "Kopiert das Profil SpoolUp Keyboard in Star Citizen, damit Tasten für Aktionen ohne Standardbelegung funktionieren. Eine Spielkopie mit eigenen Belegungen bekommt stattdessen \"SpoolUp Keyboard (your binds)\": alle deine Belegungen, dazu Tasten nur für die Deck-Aktionen, die du nicht belegt hast. Hake die Kopien an, in die es soll." } },
        { "Choose game folder...", new[] { "Elegir carpeta del juego...", "Choisir le dossier du jeu...", "Spielordner wählen..." } },
        { "STATUS", new[] { "ESTADO", "ÉTAT", "STATUS" } },
        { "Install", new[] { "Instalar", "Installer", "Installieren" } },
        { "The profile file is missing ({0}). Reinstall SpoolUp.", new[] { "Falta el archivo del perfil ({0}). Reinstala SpoolUp.", "Le fichier du profil est introuvable ({0}). Réinstallez SpoolUp.", "Die Profildatei fehlt ({0}). Installiere SpoolUp neu." } },
        { "Star Citizen wasn't found. Click \"Choose game folder...\" and pick the StarCitizen folder (the one with LIVE in it).", new[] { "No se encontró Star Citizen. Pulsa \"Elegir carpeta del juego...\" y elige la carpeta StarCitizen (la que contiene LIVE).", "Star Citizen est introuvable. Cliquez sur \"Choisir le dossier du jeu...\" et choisissez le dossier StarCitizen (celui qui contient LIVE).", "Star Citizen wurde nicht gefunden. Klicke auf \"Spielordner wählen...\" und wähle den Ordner StarCitizen (der mit LIVE darin)." } },
        { "Found {0} copy of the game. Missing one? Click \"Choose game folder...\".", new[] { "Se encontró {0} copia del juego. ¿Falta alguna? Pulsa \"Elegir carpeta del juego...\".", "{0} copie du jeu trouvée. Il en manque une ? Cliquez sur \"Choisir le dossier du jeu...\".", "{0} Spielkopie gefunden. Fehlt eine? Klicke auf \"Spielordner wählen...\"." } },
        { "Found {0} copies of the game. Missing one? Click \"Choose game folder...\".", new[] { "Se encontraron {0} copias del juego. ¿Falta alguna? Pulsa \"Elegir carpeta del juego...\".", "{0} copies du jeu trouvées. Il en manque une ? Cliquez sur \"Choisir le dossier du jeu...\".", "{0} Spielkopien gefunden. Fehlt eine? Klicke auf \"Spielordner wählen...\"." } },
        { "not installed", new[] { "no instalado", "non installé", "nicht installiert" } },
        { "already up to date", new[] { "ya actualizado", "déjà à jour", "bereits aktuell" } },
        { "older copy, will be replaced", new[] { "copia antigua, se reemplazará", "ancienne copie, sera remplacée", "ältere Kopie, wird ersetzt" } },
        { "your binds found, gets SpoolUp Keyboard (your binds)", new[] { "tus asignaciones encontradas, recibe SpoolUp Keyboard (your binds)", "vos raccourcis trouvés, reçoit SpoolUp Keyboard (your binds)", "eigene Belegungen gefunden, bekommt SpoolUp Keyboard (your binds)" } },
        { "{0} ({1} keys added)", new[] { "{0} ({1} teclas añadidas)", "{0} ({1} touches ajoutées)", "{0} ({1} Tasten hinzugefügt)" } },
        { "{0} (1 key added)", new[] { "{0} (1 tecla añadida)", "{0} (1 touche ajoutée)", "{0} (1 Taste hinzugefügt)" } },
        { "exit code {0}", new[] { "código de salida {0}", "code de sortie {0}", "Exit-Code {0}" } },
        { "Made \"SpoolUp Keyboard (your binds)\" for {0}. ", new[] { "Creado \"SpoolUp Keyboard (your binds)\" para {0}. ", "\"SpoolUp Keyboard (your binds)\" créé pour {0}. ", "\"SpoolUp Keyboard (your binds)\" erstellt für {0}. " } },
        { "Now, in Star Citizen: Options > Keybindings > Control Profiles, load it. If you change binds in the game before that, install again first. ", new[] { "Ahora, en Star Citizen: Opciones > Asignación de teclas > Perfiles de control, cárgalo. Si antes cambias asignaciones en el juego, vuelve a instalarlo primero. ", "Ensuite, dans Star Citizen : Options > Raccourcis > Profils de contrôle, chargez-le. Si vous changez des raccourcis dans le jeu avant, réinstallez-le d'abord. ", "Jetzt in Star Citizen: Optionen > Tastenbelegung > Steuerungsprofile, das Profil laden. Änderst du vorher Belegungen im Spiel, installiere es zuerst neu. " } },
        { "Your binds use some of the deck's usual keys, so free ones were picked: reload the deck on the tablet after loading the profile. ", new[] { "Tus asignaciones usan algunas teclas habituales del panel, así que se eligieron otras libres: recarga el panel en la tablet tras cargar el perfil. ", "Vos raccourcis utilisent certaines touches habituelles du panneau, d'autres touches libres ont donc été choisies : rechargez le panneau sur la tablette après avoir chargé le profil. ", "Deine Belegungen nutzen einige übliche Deck-Tasten, daher wurden freie gewählt: Lade das Deck auf dem Tablet neu, nachdem du das Profil geladen hast. " } },
        { "Pick the StarCitizen folder (the one with LIVE in it).", new[] { "Elige la carpeta StarCitizen (la que contiene LIVE).", "Choisissez le dossier StarCitizen (celui qui contient LIVE).", "Wähle den Ordner StarCitizen (der mit LIVE darin)." } },
        { "That folder doesn't look like Star Citizen. Pick the StarCitizen folder, the one with LIVE in it.", new[] { "Esa carpeta no parece de Star Citizen. Elige la carpeta StarCitizen, la que contiene LIVE.", "Ce dossier ne ressemble pas à Star Citizen. Choisissez le dossier StarCitizen, celui qui contient LIVE.", "Dieser Ordner sieht nicht nach Star Citizen aus. Wähle den Ordner StarCitizen, den mit LIVE darin." } },
        { "Added. Tick the copies of the game to install the profile into, then click Install.", new[] { "Añadido. Marca las copias del juego donde instalar el perfil y pulsa Instalar.", "Ajouté. Cochez les copies du jeu où installer le profil, puis cliquez sur Installer.", "Hinzugefügt. Hake die Spielkopien an, in die das Profil soll, und klicke auf Installieren." } },
        { "Nothing ticked. Tick the copies of the game to install the profile into.", new[] { "No hay nada marcado. Marca las copias del juego donde instalar el perfil.", "Rien n'est coché. Cochez les copies du jeu où installer le profil.", "Nichts angehakt. Hake die Spielkopien an, in die das Profil soll." } },
        { "Installed into {0}. ", new[] { "Instalado en {0}. ", "Installé dans {0}. ", "Installiert in {0}. " } },
        { "Now, in Star Citizen: Options > Keybindings > Control Profiles, load \"SpoolUp Keyboard\". ", new[] { "Ahora, en Star Citizen: Opciones > Asignación de teclas > Perfiles de control, carga \"SpoolUp Keyboard\". ", "Ensuite, dans Star Citizen : Options > Raccourcis > Profils de contrôle, chargez \"SpoolUp Keyboard\". ", "Jetzt in Star Citizen: Optionen > Tastenbelegung > Steuerungsprofile, \"SpoolUp Keyboard\" laden. " } },
        { "This replaces your current custom binds, so note any you want to keep and set them again afterwards.", new[] { "Esto sustituye tus asignaciones personalizadas actuales, así que apunta las que quieras conservar y vuelve a ponerlas después.", "Cela remplace vos raccourcis personnalisés actuels : notez ceux que vous voulez garder et refaites-les ensuite.", "Das ersetzt deine aktuellen eigenen Belegungen. Notiere dir, was du behalten willst, und stelle es danach wieder ein." } },
        { "Couldn't copy into {0}. Try running SpoolUp as administrator, or copy {1} by hand.", new[] { "No se pudo copiar en {0}. Prueba a ejecutar SpoolUp como administrador o copia {1} a mano.", "Impossible de copier dans {0}. Essayez de lancer SpoolUp en administrateur, ou copiez {1} à la main.", "Kopieren nach {0} fehlgeschlagen. Starte SpoolUp als Administrator oder kopiere {1} von Hand." } },

        // ---- Settings window ----
        { "SpoolUp settings", new[] { "Ajustes de SpoolUp", "Réglages de SpoolUp", "SpoolUp-Einstellungen" } },
        { "PAIRING", new[] { "EMPAREJAMIENTO", "APPAIRAGE", "KOPPLUNG" } },
        { "Pairing code", new[] { "Código de emparejamiento", "Code d'appairage", "Kopplungscode" } },
        { "New random code", new[] { "Nuevo código", "Nouveau code", "Neuer Code" } },
        { "Letters and numbers. Tablets already paired keep working until they reload; new ones need the new code.", new[] { "Letras y números. Las tablets ya emparejadas funcionan hasta recargar; las nuevas usan el código nuevo.", "Lettres et chiffres. Les tablettes appairées marchent jusqu'à leur rechargement, les nouvelles ont besoin du code.", "Buchstaben und Ziffern. Gekoppelte Tablets laufen bis zum Neuladen weiter; neue brauchen den neuen Code." } },
        { "Port", new[] { "Puerto", "Port", "Port" } },
        { "Set with --port when SpoolUp was started, so it can't be changed here.", new[] { "Fijado con --port al iniciar SpoolUp; no se puede cambiar aquí.", "Défini avec --port au lancement ; non modifiable ici.", "Beim Start mit --port gesetzt, daher hier nicht änderbar." } },
        { "Changing it restarts the server. Tablets then need the new address or QR code.", new[] { "Reinicia el servidor. Las tablets necesitarán la nueva dirección o el QR.", "Redémarre le serveur. Les tablettes devront reprendre l'adresse ou le QR.", "Startet den Server neu. Tablets brauchen dann neue Adresse oder QR-Code." } },
        { "KEYS", new[] { "TECLAS", "TOUCHES", "TASTEN" } },
        { "Only press keys while Star Citizen is in front", new[] { "Pulsar teclas solo con Star Citizen en primer plano", "Seulement si Star Citizen est devant", "Tasten nur drücken, wenn Star Citizen vorne ist" } },
        { "Off: keys go to whichever window is in front.", new[] { "Desactivado: las teclas van a la ventana que esté en primer plano.", "Désactivé : les touches vont à la fenêtre au premier plan.", "Aus: Tasten gehen an das Fenster, das gerade vorne ist." } },
        { "Tap length", new[] { "Duración del toque", "Durée d'appui", "Tippdauer" } },
        { "milliseconds a tap holds the key. Raise it if the game misses taps.", new[] { "milisegundos que un toque mantiene la tecla. Súbelo si el juego pierde toques.", "millisecondes de maintien par appui. À augmenter si le jeu rate des appuis.", "Millisekunden, die ein Tippen die Taste hält. Erhöhen, wenn das Spiel Tipps verpasst." } },
        { "Voice language", new[] { "Idioma de voz", "Langue vocale", "Sprache für Befehle" } },
        { "Needs that English speech recognizer installed in Windows (Settings > Time & Language > Speech).", new[] { "Necesita ese reconocedor de voz en inglés instalado en Windows (Configuración > Hora e idioma > Voz).", "Nécessite la reconnaissance vocale anglaise de Windows (Paramètres > Heure et langue > Voix).", "Braucht die englische Spracherkennung von Windows (Einstellungen > Zeit und Sprache > Sprache)." } },
        { "Emotes also answer to one word, like \"wave\"", new[] { "Gestos también con una palabra, como \"wave\"", "Emotes aussi avec un seul mot, comme \"wave\"", "Emotes reagieren auch auf ein Wort, etwa \"wave\"" } },
        { "Without this, emotes need \"emote\" first (\"emote wave\"), so a stray word doesn't set one off.", new[] { "Sin esto, los gestos necesitan \"emote\" delante (\"emote wave\"), para que una palabra suelta no active uno.", "Sans cela, les emotes demandent \"emote\" d'abord (\"emote wave\"), pour qu'un mot isolé n'en déclenche pas.", "Ohne das brauchen Emotes zuerst \"emote\" (\"emote wave\"), damit ein einzelnes Wort keins auslöst." } },
        { "PUSH-TO-TALK", new[] { "PULSAR PARA HABLAR", "APPUYER POUR PARLER", "PUSH-TO-TALK" } },
        { "Key on this PC", new[] { "Tecla en este PC", "Touche sur ce PC", "Taste an diesem PC" } },
        { "None", new[] { "Ninguna", "Aucune", "Keine" } },
        { "None: click to set", new[] { "Ninguna: clic para elegir", "Aucune : cliquez pour choisir", "Keine: zum Festlegen klicken" } },
        { "Press a key or button...", new[] { "Pulsa una tecla o botón...", "Appuyez sur une touche ou un bouton...", "Drück eine Taste oder einen Knopf..." } },
        { "Mode", new[] { "Modo", "Mode", "Modus" } },
        { "Hold to talk", new[] { "Mantener para hablar", "Maintenir pour parler", "Halten zum Sprechen" } },
        { "Press to start, again to stop", new[] { "Pulsar para empezar y otra vez para parar", "Appuyer pour démarrer, encore pour arrêter", "Drücken zum Starten, nochmal zum Beenden" } },
        { "Play a sound when it starts and stops listening", new[] { "Sonido al empezar y dejar de escuchar", "Un son quand l'écoute démarre et s'arrête", "Ton beim Start und Ende des Zuhörens" } },
        { "Pick a key Star Citizen doesn't use, like a mouse side button, a numpad key or F13-F24: the key still reaches the game.", new[] { "Elige una tecla que Star Citizen no use, como un botón lateral del ratón, una tecla del teclado numérico o F13-F24: la tecla sigue llegando al juego.", "Choisissez une touche que Star Citizen n'utilise pas, comme un bouton latéral de souris, une touche du pavé numérique ou F13-F24 : la touche arrive quand même au jeu.", "Wähle eine Taste, die Star Citizen nicht nutzt, etwa eine Maus-Seitentaste, eine Ziffernblocktaste oder F13-F24: Die Taste kommt trotzdem im Spiel an." } },
        { "LINKS", new[] { "ENLACES", "LIENS", "LINKS" } },
        { "NETWORK", new[] { "RED", "RÉSEAU", "NETZWERK" } },
        { "Tablets connect over your home network. Windows has to treat it as a Private network: on a Public one it blocks them.", new[] { "Las tablets se conectan por tu red doméstica. Windows tiene que tratarla como red privada: en una pública las bloquea.", "Les tablettes se connectent par votre réseau domestique. Windows doit le traiter comme un réseau privé : sur un réseau public, il les bloque.", "Tablets verbinden sich über dein Heimnetzwerk. Windows muss es als privates Netzwerk behandeln: In einem öffentlichen blockiert es sie." } },
        { "Let the SpoolUp app find this PC", new[] { "Permitir que la app SpoolUp encuentre este PC", "Laisser l'app SpoolUp trouver ce PC", "Die SpoolUp-App diesen PC finden lassen" } },
        { "Answers the SpoolUp app with this PC's name and address, so it shows up in the app's list.", new[] { "Responde a la app SpoolUp con el nombre y la dirección de este PC, para que aparezca en su lista.", "Répond à l'app SpoolUp avec le nom et l'adresse de ce PC, pour qu'il apparaisse dans sa liste.", "Antwortet der SpoolUp-App mit Name und Adresse dieses PCs, damit er in ihrer Liste erscheint." } },
        { "Websites the tablet can open on this PC", new[] { "Webs que la tablet puede abrir en este PC", "Sites que la tablette peut ouvrir", "Websites für das Tablet" } },
        { "One per line: Name | https://address. The tablet shows the names, and this PC opens the addresses.", new[] { "Una por línea: Nombre | https://dirección. La tablet muestra los nombres y este PC abre las direcciones.", "Un par ligne : Nom | https://adresse. La tablette affiche les noms et ce PC ouvre les adresses.", "Eine pro Zeile: Name | https://adresse. Das Tablet zeigt die Namen, dieser PC öffnet die Adressen." } },
        { "STAR CITIZEN", new[] { "STAR CITIZEN", "STAR CITIZEN", "STAR CITIZEN" } },
        { "Change...", new[] { "Cambiar...", "Changer...", "Ändern..." } },
        { "Reset to automatic", new[] { "Volver a automático", "Revenir en automatique", "Auf automatisch stellen" } },
        { "Looking for Star Citizen...", new[] { "Buscando Star Citizen...", "Recherche de Star Citizen...", "Suche Star Citizen..." } },
        { "{0}, found automatically.", new[] { "{0}, encontrado automáticamente.", "{0}, trouvé automatiquement.", "{0}, automatisch gefunden." } },
        { "{0}, in the folder you picked.", new[] { "{0}, en la carpeta que elegiste.", "{0}, dans le dossier choisi.", "{0}, im gewählten Ordner." } },
        { "Star Citizen wasn't found. Click Change... and pick the StarCitizen folder (the one with LIVE in it).", new[] { "No se encontró Star Citizen. Pulsa Cambiar... y elige la carpeta StarCitizen (la que contiene LIVE).", "Star Citizen est introuvable. Cliquez sur Changer... et choisissez le dossier StarCitizen (celui qui contient LIVE).", "Star Citizen wurde nicht gefunden. Klicke auf Ändern... und wähle den Ordner StarCitizen (der mit LIVE darin)." } },
        { "Nothing found in {0}. Click Change... to pick another folder.", new[] { "No se encontró nada en {0}. Pulsa Cambiar... para elegir otra carpeta.", "Rien trouvé dans {0}. Cliquez sur Changer... pour choisir un autre dossier.", "In {0} nichts gefunden. Klicke auf Ändern..., um einen anderen Ordner zu wählen." } },
        { "Picked {0}. Click Save to use it.", new[] { "Elegiste {0}. Pulsa Guardar para usarla.", "Dossier choisi : {0}. Cliquez sur Enregistrer pour l'utiliser.", "{0} gewählt. Klicke auf Speichern, um ihn zu nutzen." } },
        { "Back to automatic. Click Save to use it.", new[] { "De vuelta a automático. Pulsa Guardar para usarlo.", "Retour en automatique. Cliquez sur Enregistrer pour l'appliquer.", "Wieder automatisch. Klicke auf Speichern, um es zu nutzen." } },
        { "Keybinds, the keybind profile and battle alerts use this copy of the game.", new[] { "Las teclas, el perfil de teclas y las alertas de combate usan esta copia del juego.", "Les touches, le profil de touches et les alertes de combat utilisent cette copie du jeu.", "Tastenbelegung, Tastenprofil und Kampfmeldungen nutzen diese Kopie des Spiels." } },
        { "Install keybind profile...", new[] { "Instalar perfil de teclas...", "Installer le profil de touches...", "Tastenprofil installieren..." } },
        { "Language", new[] { "Idioma", "Langue", "Sprache" } },
        { "Automatic", new[] { "Automático", "Automatique", "Automatisch" } },
        { "Save", new[] { "Guardar", "Enregistrer", "Speichern" } },
        { "Cancel", new[] { "Cancelar", "Annuler", "Abbrechen" } },
        { "This link isn't right:\n\n{0}\n\nWrite each one as: Name | https://address (names up to 24 characters).", new[] { "Este enlace no es correcto:\n\n{0}\n\nEscribe cada uno así: Nombre | https://dirección (nombres de hasta 24 caracteres).", "Ce lien n'est pas correct :\n\n{0}\n\nÉcrivez chacun ainsi : Nom | https://adresse (noms de 24 caractères maximum).", "Dieser Link stimmt nicht:\n\n{0}\n\nSchreib jeden so: Name | https://adresse (Namen bis 24 Zeichen)." } },
        { "Up to 12 links, please.", new[] { "Como máximo 12 enlaces, por favor.", "12 liens au maximum, s'il vous plaît.", "Höchstens 12 Links, bitte." } },
        { "The pairing code can only have letters and numbers (up to 32).", new[] { "El código de emparejamiento solo puede tener letras y números (hasta 32).", "Le code d'appairage ne peut contenir que des lettres et des chiffres (32 au maximum).", "Der Kopplungscode darf nur Buchstaben und Ziffern haben (bis zu 32)." } },
        { "The voice language should look like en-US.", new[] { "El idioma de voz debe tener la forma en-US.", "La langue vocale doit ressembler à en-US.", "Die Sprache für Befehle sollte wie en-US aussehen." } },
    };
}

// The app window's colors and fonts (docs/DESIGN.md) and its drawing pieces: solid panels
// with a title, letter-spaced labels, status dots and pills. Text sits on solid fills only,
// in Ink or Dim; the status colors mark a dot, a pill or a banner, never body text.
// Sizes are in 96-dpi pixels; HudView scales everything for the screen.
static class Hud
{
    [DllImport("dwmapi.dll")]
    static extern int DwmSetWindowAttribute(IntPtr hwnd, int attribute, ref int value, int size);

    [DllImport("uxtheme.dll", CharSet = CharSet.Unicode)]
    static extern int SetWindowTheme(IntPtr hwnd, string app, string idList);

    // A dark title bar (Windows 10 2004 and later use attribute 20, earlier builds 19).
    public static void DarkTitleBar(Form form)
    {
        int on = 1;
        try
        {
            if (DwmSetWindowAttribute(form.Handle, 20, ref on, 4) != 0) DwmSetWindowAttribute(form.Handle, 19, ref on, 4);
        }
        catch (Exception) { }
    }

    // Dark scrollbars on a list or text box (Windows 10 1809 and later).
    public static void DarkScrollbars(Control c)
    {
        c.HandleCreated += delegate { try { SetWindowTheme(c.Handle, "DarkMode_Explorer", null); } catch (Exception) { } };
    }

    // A graphite palette: solid surfaces a step apart, near-white text, one blue accent.
    // Dim is still 7:1 on Panel, so secondary text stays easy to read.
    public static readonly Color Back = Color.FromArgb(0x0E, 0x11, 0x16);    // window, and fields inside panels
    public static readonly Color Panel = Color.FromArgb(0x16, 0x1B, 0x22);
    public static readonly Color PanelHi = Color.FromArgb(0x1F, 0x26, 0x30); // buttons, hover
    public static readonly Color Line = Color.FromArgb(0x2D, 0x36, 0x43);
    public static readonly Color Ink = Color.FromArgb(0xF2, 0xF5, 0xF8);
    public static readonly Color Dim = Color.FromArgb(0xAA, 0xB4, 0xC0);
    public static readonly Color Accent = Color.FromArgb(0x4F, 0xB3, 0xFF); // buttons, links, focus
    public static readonly Color AccentInk = Color.FromArgb(0x06, 0x12, 0x1F); // text on Accent
    public static readonly Color Good = Color.FromArgb(0x3D, 0xDC, 0x84);
    public static readonly Color Warn = Color.FromArgb(0xFF, 0xB0, 0x20);
    public static readonly Color WarnInk = Color.FromArgb(0x1A, 0x13, 0x00);   // text on Warn
    public static readonly Color Bad = Color.FromArgb(0xFF, 0x5A, 0x5F);

    // Bahnschrift (Windows 10 and later) has the squared, instrument-panel look of the deck's
    // fonts; older Windows falls back to Segoe UI. Labels use its SemiBold cut: the light and
    // regular weights look thin at these sizes.
    static readonly string Display = Installed("Bahnschrift") ? "Bahnschrift" : "Segoe UI";
    static readonly string DisplaySemi = Installed("Bahnschrift SemiBold") ? "Bahnschrift SemiBold" : null;
    static readonly Dictionary<string, Font> fonts = new Dictionary<string, Font>();
    static readonly StringFormat typographic = MakeTypographic();

    // GDI+ quietly swaps in another font for a family it doesn't have, so ask first.
    static bool Installed(string family)
    {
        try
        {
            using (var f = new FontFamily(family))
                return f.Name == family && f.IsStyleAvailable(FontStyle.Regular);
        }
        catch (Exception) { return false; }
    }

    static StringFormat MakeTypographic()
    {
        var f = (StringFormat)StringFormat.GenericTypographic.Clone();
        f.FormatFlags |= StringFormatFlags.MeasureTrailingSpaces | StringFormatFlags.NoWrap;
        return f;
    }

    static Font Get(string family, float px, FontStyle style)
    {
        string key = family + px + style;
        Font f;
        if (fonts.TryGetValue(key, out f)) return f;
        // A family or style this PC lacks falls back to Segoe UI, then to the default font.
        foreach (var name in new[] { family, "Segoe UI" })
        {
            try { return fonts[key] = new Font(name, px, style, GraphicsUnit.Pixel); }
            catch (Exception) { }
        }
        return fonts[key] = new Font(SystemFonts.DefaultFont.FontFamily, px, style, GraphicsUnit.Pixel);
    }

    public static Font Head(float px) { return Get(Display, px, FontStyle.Bold); }
    public static Font Label(float px) { return DisplaySemi != null ? Get(DisplaySemi, px, FontStyle.Regular) : Get(Display, px, FontStyle.Bold); }
    public static Font Body(float px) { return Get("Segoe UI", px, FontStyle.Regular); }
    public static Font Mono(float px, bool bold) { return Get("Consolas", px, bold ? FontStyle.Bold : FontStyle.Regular); }

    // Screen pixels per 96-dpi pixel, read from the screen itself so no window has to exist yet.
    public static float ScreenZoom()
    {
        try { using (var g = Graphics.FromHwnd(IntPtr.Zero)) return Math.Max(1f, g.DpiX / 96f); }
        catch (Exception) { return 1f; }
    }

    public static Color Fade(Color c, int alpha) { return Color.FromArgb(Math.Max(0, Math.Min(255, alpha)), c); }

    // a mixed with b: t = 0 is a, 1 is b. Opaque, unlike Fade, so text over it keeps ClearType.
    public static Color Mix(Color a, Color b, float t)
    {
        return Color.FromArgb((int)(a.R + (b.R - a.R) * t), (int)(a.G + (b.G - a.G) * t), (int)(a.B + (b.B - a.B) * t));
    }

    // Letter-spaced text, the labels and headings; returns its width.
    public static float Spaced(Graphics g, string text, Font font, Color color, float x, float y, float spacing)
    {
        float start = x, scale;
        using (var m = g.Transform) scale = m.Elements[0]; // screen pixels per unit
        float top = (float)Math.Round(y * scale) / scale;
        using (var brush = new SolidBrush(color))
            foreach (char ch in text)
            {
                string s = ch.ToString();
                // Each letter starts on a whole screen pixel; fractional positions smear small text.
                g.DrawString(s, font, brush, (float)Math.Round(x * scale) / scale, top, typographic);
                x += g.MeasureString(s, font, PointF.Empty, typographic).Width + spacing;
            }
        return x - start - (text.Length > 0 ? spacing : 0);
    }

    public static float SpacedWidth(Graphics g, string text, Font font, float spacing)
    {
        float w = 0;
        foreach (char ch in text) w += g.MeasureString(ch.ToString(), font, PointF.Empty, typographic).Width + spacing;
        return w - (text.Length > 0 ? spacing : 0);
    }

    public static float Width(Graphics g, string text, Font font)
    {
        return g.MeasureString(text, font, PointF.Empty, typographic).Width;
    }

    // One line of text, cut short with "..." when it doesn't fit.
    public static void Text(Graphics g, string text, Font font, Color color, RectangleF box)
    {
        using (var brush = new SolidBrush(color))
        using (var f = (StringFormat)typographic.Clone())
        {
            f.Trimming = StringTrimming.EllipsisCharacter;
            f.LineAlignment = StringAlignment.Center;
            g.DrawString(text, font, brush, box, f);
        }
    }

    // Text that wraps inside the box.
    public static void Wrapped(Graphics g, string text, Font font, Color color, RectangleF box)
    {
        using (var brush = new SolidBrush(color))
        using (var f = new StringFormat { Trimming = StringTrimming.EllipsisWord })
            g.DrawString(text, font, brush, box, f);
    }

    // The height wrapped text takes at a width, for layouts that grow with their text.
    public static float WrappedHeight(string text, Font font, float width)
    {
        using (var bmp = new Bitmap(1, 1))
        using (var g = Graphics.FromImage(bmp))
            return (float)Math.Ceiling(g.MeasureString(text, font, (int)width).Height);
    }

    // A status dot: solid, no glow.
    public static void Dot(Graphics g, Color color, float cx, float cy)
    {
        using (var dot = new SolidBrush(color)) g.FillEllipse(dot, cx - 4, cy - 4, 8, 8);
    }

    public static GraphicsPath Rounded(RectangleF r, float radius)
    {
        var path = new GraphicsPath();
        float d = Math.Min(radius * 2, Math.Min(r.Width, r.Height));
        if (d <= 0) { path.AddRectangle(r); return path; }
        path.AddArc(r.X, r.Y, d, d, 180, 90);
        path.AddArc(r.Right - d, r.Y, d, d, 270, 90);
        path.AddArc(r.Right - d, r.Bottom - d, d, d, 0, 90);
        path.AddArc(r.X, r.Bottom - d, d, d, 90, 90);
        path.CloseFigure();
        return path;
    }

    public static void Box(Graphics g, RectangleF r, float radius, Color fill, Color border)
    {
        using (var path = Rounded(r, radius))
        {
            if (fill != Color.Empty) using (var brush = new SolidBrush(fill)) g.FillPath(brush, path);
            if (border != Color.Empty) using (var pen = new Pen(border, 1f)) g.DrawPath(pen, path);
        }
    }

    // A status pill ending at right: a dot and a word on a tint of the status color, with
    // Ink text so it reads on any of them. Returns its box.
    public static RectangleF Pill(Graphics g, string word, Color color, float right, float y)
    {
        var font = Label(13);
        float w = SpacedWidth(g, word, font, 1.4f);
        var r = new RectangleF(right - w - 40, y, w + 40, 26);
        Box(g, r, 13, Mix(Panel, color, 0.22f), Mix(Line, color, 0.6f));
        Dot(g, color, r.X + 15, r.Y + 13);
        Spaced(g, word, font, Ink, r.X + 26, r.Y + (r.Height - font.Height) / 2 + 1, 1.4f);
        return r;
    }

    // A panel: a solid fill, a thin border and its title, Ink on a short accent bar.
    public static void PanelBox(Graphics g, RectangleF r, string title)
    {
        Box(g, r, 6, Panel, Line);
        using (var bar = new SolidBrush(Accent)) g.FillRectangle(bar, r.X + 16, r.Y + 15, 3, 12);
        Spaced(g, Lang.T(title), Head(12.5f), Ink, r.X + 26, r.Y + 13, 1.4f);
    }
}

// The main window's face, drawn by hand: solid panels on a solid background, the window's
// own buttons, switches, slider and links, and a thin band of stars in the title strip that
// drifts, shifts with the mouse (near stars more than far ones) and lights up along a ring
// sent out by a click. The owner draws the panels in DrawContent onto an opaque layer, so
// text gets ClearType; that layer is cached and only redrawn after Redraw(). The stars are
// drawn over it only inside StarBand and outside Covers, so they never pass behind text.
// They move only while Running (the window is in front) and Windows' "Show animations" is
// on, so nothing animates behind Star Citizen.
class HudView : Control
{
    public enum Kind { Button, Primary, Switch, Slider, Link }

    // The item sits on the amber warning banner: drawn dark, so it reads on amber.
    public const int OnWarn = 1;

    // Something to click. Box is in 96-dpi pixels, like everything drawn here.
    public class Item
    {
        public Kind Kind;
        public RectangleF Box;
        public string Text = "", Tip = "";
        public bool Visible = true, Enabled = true, On;
        public bool Mono; // links: the address, in a monospaced font
        public int Look;  // 0, or OnWarn
        public int Value, Min, Max = 100;
        public MethodInvoker Click;   // buttons and links; switches after On flips
        public MethodInvoker Changed; // the slider, on every step
        public Action<MouseButtons> OtherButton; // a middle or side mouse button pressed on it
    }

    [DllImport("user32.dll")]
    static extern bool SystemParametersInfo(int action, int param, ref int value, int winIni);

    class Star { public float X, Y, Depth, Size, Phase, Glow; public int Alpha; }
    class Pulse { public float X, Y, Age; }

    public readonly List<Item> Items = new List<Item>();
    public Action<Graphics> DrawContent;
    // The only place stars are drawn (empty: none), and the text inside it they stay out
    // of. DrawContent sets Covers; the items are covered too.
    public RectangleF StarBand = RectangleF.Empty;
    public readonly List<RectangleF> Covers = new List<RectangleF>();
    public float Zoom = 1f; // screen pixels per 96-dpi pixel

    readonly List<Star> stars = new List<Star>();
    readonly List<Pulse> pulses = new List<Pulse>();
    readonly System.Windows.Forms.Timer frame = new System.Windows.Forms.Timer { Interval = 33 };
    readonly Stopwatch clock = Stopwatch.StartNew();
    readonly ToolTip tips = new ToolTip();
    readonly System.Windows.Forms.Timer tipDelay = new System.Windows.Forms.Timer { Interval = 500 };
    double lastFrame;
    float time;
    PointF mouse, look; // the mouse, and where the stars currently "look" (eases toward it)
    bool mouseInside;
    Bitmap backdrop, content;
    bool contentDirty = true;
    bool running, motion = true;
    Item hover, pressed, dragging, focused;
    bool showFocus;

    public HudView()
    {
        SetStyle(ControlStyles.OptimizedDoubleBuffer | ControlStyles.AllPaintingInWmPaint | ControlStyles.UserPaint
            | ControlStyles.ResizeRedraw | ControlStyles.Selectable, true);
        TabStop = true;
        var random = new Random(7);
        for (int i = 0; i < 240; i++)
        {
            float depth = i % 7 == 0 ? 1f : i % 3 == 0 ? 0.6f : 0.3f;
            stars.Add(new Star
            {
                X = (float)random.NextDouble(), Y = (float)random.NextDouble(), Depth = depth,
                Size = depth >= 1f ? 2.6f : depth >= 0.6f ? 1.9f : 1.3f,
                Phase = (float)(random.NextDouble() * Math.PI * 2),
                Alpha = (int)(120 + 135 * depth * (0.5 + 0.5 * random.NextDouble())),
            });
        }
        frame.Tick += delegate { Invalidate(); };
        tipDelay.Tick += delegate
        {
            tipDelay.Stop();
            if (hover != null && hover.Tip != "") tips.Show(Lang.T(hover.Tip), this, (int)(mouse.X * Zoom) + 14, (int)(mouse.Y * Zoom) + 22, 6000);
        };
    }

    // Design size: the client area in 96-dpi pixels.
    public SizeF Area { get { return new SizeF(ClientSize.Width / Zoom, ClientSize.Height / Zoom); } }

    // The window is in front: animate, if Windows' animations are on.
    public bool Running
    {
        get { return running; }
        set
        {
            running = value;
            motion = AnimationsOn();
            if (running && motion) { lastFrame = clock.Elapsed.TotalSeconds; frame.Start(); }
            else { frame.Stop(); pulses.Clear(); }
            Invalidate();
        }
    }

    // "Show animations in Windows" (Settings > Accessibility > Visual effects).
    static bool AnimationsOn()
    {
        int on = 1;
        try { SystemParametersInfo(0x1042, 0, ref on, 0); } // SPI_GETCLIENTAREAANIMATION
        catch (Exception) { }
        return on != 0;
    }

    // The panels changed: draw them again on the next frame.
    public void Redraw()
    {
        contentDirty = true;
        Invalidate();
    }

    public Item Add(Kind kind, string text, MethodInvoker click)
    {
        var item = new Item { Kind = kind, Text = text, Click = click };
        Items.Add(item);
        return item;
    }

    protected override void OnSizeChanged(EventArgs e)
    {
        base.OnSizeChanged(e);
        if (backdrop != null) { backdrop.Dispose(); backdrop = null; }
        if (ClientSize.Width > 0 && ClientSize.Height > 0) Redraw();
    }

    // ---------- drawing ----------

    protected override void OnPaint(PaintEventArgs e)
    {
        if (ClientSize.Width < 1 || ClientSize.Height < 1) return;
        double now = clock.Elapsed.TotalSeconds;
        float dt = (float)Math.Min(0.1, now - lastFrame);
        lastFrame = now;
        if (running && motion) Step(dt);

        var g = e.Graphics;
        if (contentDirty || content == null) DrawContentLayer();
        g.DrawImageUnscaled(content, 0, 0);
        if (StarBand.IsEmpty) return;
        g.ScaleTransform(Zoom, Zoom);
        g.SmoothingMode = SmoothingMode.AntiAlias;
        using (var open = new Region(StarBand))
        {
            foreach (var r in Covers) open.Exclude(RectangleF.Inflate(r, 1, 1));
            foreach (var item in Items) if (item.Visible) open.Exclude(RectangleF.Inflate(item.Box, 2, 2));
            g.Clip = open;
            DrawStars(g);
        }
    }

    void Step(float dt)
    {
        time += dt;
        var area = Area;
        var target = mouseInside ? mouse : new PointF(area.Width / 2, area.Height / 2);
        float ease = Math.Min(1f, dt * 3f);
        look = new PointF(look.X + (target.X - look.X) * ease, look.Y + (target.Y - look.Y) * ease);
        foreach (var s in stars)
        {
            s.X -= 0.0035f * s.Depth * dt;
            if (s.X < -0.02f) s.X += 1.04f;
            s.Glow = Math.Max(0, s.Glow - dt * 1.1f);
        }
        for (int i = pulses.Count - 1; i >= 0; i--)
        {
            pulses[i].Age += dt;
            if (pulses[i].Age > 1.8f) pulses.RemoveAt(i);
        }
    }

    // The background: one solid color, nothing behind the text.
    Bitmap DrawBackdrop()
    {
        var bmp = new Bitmap(ClientSize.Width, ClientSize.Height, PixelFormat.Format32bppPArgb);
        using (var g = Graphics.FromImage(bmp)) g.Clear(Hud.Back);
        return bmp;
    }

    void DrawStars(Graphics g)
    {
        var area = Area;
        float cx = area.Width / 2, cy = area.Height / 2;
        if (look.IsEmpty) look = new PointF(cx, cy);
        // A third of them: the band is thin, and a sparse field stays calm.
        for (int i = 0; i < stars.Count; i += 3)
        {
            var s = stars[i];
            float x = s.X * area.Width - (look.X - cx) * 0.035f * s.Depth;
            float y = StarBand.Y + s.Y * StarBand.Height - (look.Y - cy) * 0.01f * s.Depth;
            foreach (var p in pulses)
            {
                float d = (float)Math.Sqrt((x - p.X) * (x - p.X) + (y - p.Y) * (y - p.Y));
                if (Math.Abs(d - PulseRadius(p)) < 16) s.Glow = 1f;
            }
            float twinkle = motion ? 0.78f + 0.22f * (float)Math.Sin(time * (1.3 + s.Depth) + s.Phase) : 1f;
            int alpha = (int)(s.Alpha * twinkle + (255 - s.Alpha) * s.Glow);
            var color = s.Glow > 0 ? Hud.Mix(Hud.Ink, Hud.Accent, s.Glow) : (s.Depth >= 1f ? Hud.Ink : Hud.Dim);
            float size = s.Size + s.Glow * 2.5f;
            // Near stars and lit ones get a soft halo.
            if (s.Depth >= 1f || s.Glow > 0)
                using (var halo = new SolidBrush(Hud.Fade(color, alpha / 5)))
                    g.FillEllipse(halo, x - size * 1.6f, y - size * 1.6f, size * 3.2f, size * 3.2f);
            using (var brush = new SolidBrush(Hud.Fade(color, alpha)))
                g.FillEllipse(brush, x - size / 2, y - size / 2, size, size);
        }
        foreach (var p in pulses)
        {
            float r = PulseRadius(p), fade = 1f - p.Age / 1.8f;
            using (var pen = new Pen(Hud.Fade(Hud.Accent, (int)(150 * fade)), 1.5f)) g.DrawEllipse(pen, p.X - r, p.Y - r, r * 2, r * 2);
            using (var pen = new Pen(Hud.Fade(Hud.Accent, (int)(60 * fade)), 1f)) g.DrawEllipse(pen, p.X - r * 0.8f, p.Y - r * 0.8f, r * 1.6f, r * 1.6f);
        }
    }

    static float PulseRadius(Pulse p) { return p.Age * 460f; }

    void DrawContentLayer()
    {
        if (content == null || content.Width != ClientSize.Width || content.Height != ClientSize.Height)
        {
            if (content != null) content.Dispose();
            content = new Bitmap(ClientSize.Width, ClientSize.Height, PixelFormat.Format32bppRgb);
        }
        if (backdrop == null) backdrop = DrawBackdrop();
        Covers.Clear();
        using (var g = Graphics.FromImage(content))
        {
            g.DrawImageUnscaled(backdrop, 0, 0); // opaque, so the text below can use ClearType
            g.ScaleTransform(Zoom, Zoom);
            g.SmoothingMode = SmoothingMode.AntiAlias;
            g.TextRenderingHint = TextRenderingHint.ClearTypeGridFit;
            if (DrawContent != null) DrawContent(g);
            foreach (var item in Items) if (item.Visible) DrawItem(g, item);
        }
        contentDirty = false;
    }

    void DrawItem(Graphics g, Item item)
    {
        var r = item.Box;
        bool hot = item == hover && item.Enabled, down = item == pressed && hot;
        bool onWarn = item.Look == OnWarn;
        switch (item.Kind)
        {
            case Kind.Button:
            case Kind.Primary:
                // Primary: solid accent with dark text. The others: a raised gray fill with a
                // border. On the warning banner: dark on amber.
                bool primary = item.Kind == Kind.Primary;
                Color fill, border, ink;
                if (!item.Enabled) { fill = Hud.Panel; border = Hud.Line; ink = Hud.Mix(Hud.Panel, Hud.Dim, 0.6f); }
                else if (onWarn) { fill = down ? Hud.Mix(Hud.WarnInk, Hud.Warn, 0.25f) : hot ? Hud.Mix(Hud.WarnInk, Hud.Warn, 0.14f) : Hud.WarnInk; border = fill; ink = Hud.Ink; }
                else if (primary) { fill = down ? Hud.Mix(Hud.Accent, Hud.AccentInk, 0.2f) : hot ? Hud.Mix(Hud.Accent, Hud.Ink, 0.25f) : Hud.Accent; border = fill; ink = Hud.AccentInk; }
                else { fill = down ? Hud.Line : hot ? Hud.Mix(Hud.PanelHi, Hud.Line, 0.6f) : Hud.PanelHi; border = hot ? Hud.Mix(Hud.Line, Hud.Accent, 0.5f) : Hud.Line; ink = Hud.Ink; }
                Hud.Box(g, r, 5, fill, border);
                var font = Hud.Label(13.5f);
                string text = Lang.T(item.Text);
                float w = Hud.SpacedWidth(g, text, font, 0.4f);
                // A longer translation gets a smaller font rather than spilling out of the button.
                for (float px = 13f; w > r.Width - 20 && px >= 10.5f; px -= 0.5f)
                {
                    font = Hud.Label(px);
                    w = Hud.SpacedWidth(g, text, font, 0.4f);
                }
                Hud.Spaced(g, text, font, ink, r.X + (r.Width - w) / 2, r.Y + (r.Height - font.Height) / 2 + 1, 0.4f);
                break;
            case Kind.Switch:
                var track = new RectangleF(r.X, r.Y + (r.Height - 20) / 2, 36, 20);
                Hud.Box(g, track, 10, item.On ? (hot ? Hud.Mix(Hud.Accent, Hud.Ink, 0.2f) : Hud.Accent) : Hud.Back,
                    item.On ? Color.Empty : hot ? Hud.Accent : Hud.Mix(Hud.Line, Hud.Dim, 0.35f));
                using (var knob = new SolidBrush(item.On ? Hud.AccentInk : Hud.Dim))
                    g.FillEllipse(knob, item.On ? track.Right - 17 : track.X + 3, track.Y + 3, 14, 14);
                // Two lines where it needs them, rather than cutting the label short.
                var label = new RectangleF(r.X + 48, r.Y, r.Width - 48, r.Height);
                string sw = Lang.T(item.Text);
                if (Hud.Width(g, sw, Hud.Body(13)) <= label.Width) Hud.Text(g, sw, Hud.Body(13), Hud.Ink, label);
                else
                    using (var brush = new SolidBrush(Hud.Ink))
                    using (var f = new StringFormat { LineAlignment = StringAlignment.Center, Trimming = StringTrimming.EllipsisWord })
                        g.DrawString(sw, Hud.Body(13), brush, label, f);
                break;
            case Kind.Slider:
                float mid = r.Y + r.Height / 2, t = item.Max > item.Min ? (item.Value - item.Min) / (float)(item.Max - item.Min) : 0;
                float knobX = r.X + 7 + t * (r.Width - 14);
                Hud.Box(g, new RectangleF(r.X, mid - 2, r.Width, 4), 2, Hud.Line, Color.Empty);
                Hud.Box(g, new RectangleF(r.X, mid - 2, knobX - r.X, 4), 2, item.Enabled ? Hud.Accent : Hud.Dim, Color.Empty);
                using (var brush = new SolidBrush(!item.Enabled ? Hud.Dim : hot || dragging == item ? Hud.Ink : Hud.Accent))
                    g.FillEllipse(brush, knobX - 7, mid - 7, 14, 14);
                using (var pen = new Pen(Hud.Panel, 2f)) g.DrawEllipse(pen, knobX - 7, mid - 7, 14, 14);
                break;
            case Kind.Link:
                var linkFont = item.Mono ? Hud.Mono(18, true) : Hud.Body(13);
                string link = Lang.T(item.Text);
                float lw = Hud.Width(g, link, linkFont);
                r.Width = Math.Min(lw + 2, item.Box.Width);
                Hud.Text(g, link, linkFont, hot ? Hud.Ink : Hud.Accent, item.Box);
                using (var pen = new Pen(hot ? Hud.Ink : Hud.Fade(Hud.Accent, 110), 1f)) g.DrawLine(pen, r.X, r.Bottom - 1, r.Right, r.Bottom - 1);
                break;
        }
        // A keyboard focus ring that's easy to see: a solid 2-pixel outline around the item.
        if (item == focused && showFocus && Focused)
            using (var path = Hud.Rounded(RectangleF.Inflate(r, 3, 3), 7))
            using (var pen = new Pen(onWarn ? Hud.WarnInk : Hud.Accent, 2f))
                g.DrawPath(pen, path);
    }

    // The QR code at whole screen pixels per module, so cameras read it easily, with a white
    // margin of four modules.
    public void DrawQr(Graphics g, RectangleF box, string[] rows)
    {
        var state = g.Save();
        g.ResetTransform();
        g.SmoothingMode = SmoothingMode.None;
        int n = rows.Length;
        int side = (int)(Math.Min(box.Width, box.Height) * Zoom);
        int module = Math.Max(1, side / (n + 8));
        int size = module * (n + 8);
        int x0 = (int)(box.X * Zoom) + ((int)(box.Width * Zoom) - size) / 2, y0 = (int)(box.Y * Zoom) + ((int)(box.Height * Zoom) - size) / 2;
        g.FillRectangle(Brushes.White, x0, y0, size, size);
        for (int y = 0; y < n; y++)
            for (int x = 0; x < rows[y].Length; x++)
                if (rows[y][x] == '1') g.FillRectangle(Brushes.Black, x0 + (x + 4) * module, y0 + (y + 4) * module, module, module);
        g.Restore(state);
    }

    // ---------- mouse and keyboard ----------

    Item ItemAt(PointF p)
    {
        for (int i = Items.Count - 1; i >= 0; i--)
        {
            var item = Items[i];
            if (item.Visible && item.Enabled && RectangleF.Inflate(item.Box, 0, item.Kind == Kind.Slider ? 6 : 0).Contains(p)) return item;
        }
        return null;
    }

    protected override void OnMouseMove(MouseEventArgs e)
    {
        base.OnMouseMove(e);
        mouse = new PointF(e.X / Zoom, e.Y / Zoom);
        mouseInside = true;
        if (dragging != null) { SlideTo(dragging, mouse.X); return; }
        var now = ItemAt(mouse);
        if (now == hover) return;
        hover = now;
        Cursor = hover != null ? Cursors.Hand : Cursors.Default;
        tips.Hide(this);
        tipDelay.Stop();
        if (hover != null && hover.Tip != "") tipDelay.Start();
        Redraw();
    }

    protected override void OnMouseLeave(EventArgs e)
    {
        base.OnMouseLeave(e);
        mouseInside = false;
        tips.Hide(this);
        tipDelay.Stop();
        if (hover != null) { hover = null; Cursor = Cursors.Default; Redraw(); }
    }

    protected override void OnMouseDown(MouseEventArgs e)
    {
        base.OnMouseDown(e);
        Focus();
        showFocus = false;
        var p = new PointF(e.X / Zoom, e.Y / Zoom);
        var item = ItemAt(p);
        if (e.Button != MouseButtons.Left)
        {
            if (item != null && item.OtherButton != null) item.OtherButton(e.Button);
            return;
        }
        tips.Hide(this);
        if (item == null)
        {
            // An empty spot sends a pulse through the stars.
            if (running && motion) pulses.Add(new Pulse { X = p.X, Y = p.Y });
            return;
        }
        pressed = focused = item;
        if (item.Kind == Kind.Slider) { dragging = item; SlideTo(item, p.X); }
        Redraw();
    }

    protected override void OnMouseUp(MouseEventArgs e)
    {
        base.OnMouseUp(e);
        var p = new PointF(e.X / Zoom, e.Y / Zoom);
        var item = pressed;
        pressed = null;
        dragging = null;
        if (item != null && item.Kind != Kind.Slider && ItemAt(p) == item) Activate(item);
        Redraw();
    }

    void SlideTo(Item item, float x)
    {
        float t = Math.Max(0, Math.Min(1, (x - item.Box.X - 7) / (item.Box.Width - 14)));
        SetValue(item, item.Min + (int)Math.Round(t * (item.Max - item.Min)));
    }

    void SetValue(Item item, int value)
    {
        value = Math.Max(item.Min, Math.Min(item.Max, value));
        if (value == item.Value) return;
        item.Value = value;
        if (item.Changed != null) item.Changed();
        Redraw();
    }

    void Activate(Item item)
    {
        if (item.Kind == Kind.Switch) item.On = !item.On;
        if (item.Click != null) item.Click();
        Redraw();
    }

    // Tab and Shift+Tab move between the items, Enter or Space presses one, and the arrow
    // keys move the slider.
    protected override bool ProcessDialogKey(Keys keyData)
    {
        var key = keyData & Keys.KeyCode;
        if (key != Keys.Tab || (keyData & (Keys.Control | Keys.Alt)) != 0) return base.ProcessDialogKey(keyData);
        var usable = Items.FindAll(i => i.Visible && i.Enabled);
        int at = usable.IndexOf(focused) + ((keyData & Keys.Shift) != 0 ? -1 : 1);
        if (focused == null || at < 0 || at >= usable.Count)
        {
            // Past the last item (or before the first): on to the window's other controls.
            focused = null;
            Redraw();
            return base.ProcessDialogKey(keyData);
        }
        focused = usable[at];
        showFocus = true;
        Redraw();
        return true;
    }

    // Arrow keys move the slider; Enter and Space press the focused item instead of the
    // window's default button.
    protected override bool IsInputKey(Keys keyData)
    {
        var key = keyData & Keys.KeyCode;
        if ((key == Keys.Enter || key == Keys.Space) && focused != null && showFocus) return true;
        return key == Keys.Left || key == Keys.Right || key == Keys.Up || key == Keys.Down || base.IsInputKey(keyData);
    }

    protected override void OnKeyDown(KeyEventArgs e)
    {
        base.OnKeyDown(e);
        if (focused == null || !focused.Visible || !focused.Enabled) return;
        if (focused.Kind == Kind.Slider)
        {
            int step = e.Shift ? 5 : 1;
            if (e.KeyCode == Keys.Left || e.KeyCode == Keys.Down) SetValue(focused, focused.Value - step);
            else if (e.KeyCode == Keys.Right || e.KeyCode == Keys.Up) SetValue(focused, focused.Value + step);
        }
        else if (e.KeyCode == Keys.Enter || e.KeyCode == Keys.Space) Activate(focused);
    }

    // When Tab was last pressed anywhere in the app. Windows also moves focus "by Tab" when a
    // window opens, so only a real Tab press shows the focus ring.
    static DateTime lastTab = DateTime.MinValue;

    class TabWatch : IMessageFilter
    {
        public bool PreFilterMessage(ref Message m)
        {
            if (m.Msg == 0x0100 && (int)m.WParam == 0x09) lastTab = DateTime.Now; // WM_KEYDOWN, VK_TAB
            return false;
        }
    }

    static HudView() { Application.AddMessageFilter(new TabWatch()); }

    // Tabbed into from another control: start at the first item, or the last with Shift.
    // Focus that comes with the window opening or a click shows no focus ring.
    protected override void Select(bool directed, bool forward)
    {
        var usable = Items.FindAll(i => i.Visible && i.Enabled);
        if (directed && usable.Count > 0 && (DateTime.Now - lastTab).TotalMilliseconds < 500)
        {
            focused = forward ? usable[0] : usable[usable.Count - 1];
            showFocus = true;
        }
        base.Select(directed, forward);
        Redraw();
    }

    protected override void OnGotFocus(EventArgs e) { base.OnGotFocus(e); Redraw(); }
    protected override void OnLostFocus(EventArgs e) { base.OnLostFocus(e); Redraw(); }
}

// The other windows' look: the main window's panels, buttons and switches, drawn by a
// HudView on the same solid background, without the stars. Fields that take typing or
// picking (text boxes, number boxes, lists) stay real Windows controls on top, in the
// window's colors inside a drawn frame. Positions are in 96-dpi pixels, like HudView's.
class HudDialog : Form
{
    // Text drawn in a panel: a caption, a hint or a status line. Change Text, Color or Mark,
    // then Redraw. A status line keeps its text Ink and shows its state as a dot (Mark).
    protected class Note
    {
        public string Text = "";
        public Color Color = Hud.Dim;
        public Color Mark = Color.Empty;
        public RectangleF Box;
        public Font Font;
        public bool Caption;
    }

    protected readonly HudView hud = new HudView();
    // A panel's title and box; set Box again in Arrange to move it.
    protected class PanelPart
    {
        public string Title;
        public RectangleF Box;
    }

    readonly List<PanelPart> panels = new List<PanelPart>();
    readonly List<Note> notes = new List<Note>();
    readonly List<Control> fields = new List<Control>();
    readonly Dictionary<Control, RectangleF> frames = new Dictionary<Control, RectangleF>();
    // Enter and Esc, the window's default and cancel buttons.
    protected MethodInvoker Accept, Cancel;

    protected HudDialog(string title, Icon icon, int width, int height)
    {
        SuspendLayout();
        Text = Lang.T(title);
        Icon = icon;
        ShowInTaskbar = false;
        BackColor = Hud.Back;
        ForeColor = Hud.Ink;
        Font = new Font("Segoe UI", 9.75f);
        AutoScaleMode = AutoScaleMode.None; // HudView scales itself, and Place the fields
        hud.Zoom = Hud.ScreenZoom();
        ClientSize = new Size((int)(width * hud.Zoom), (int)(height * hud.Zoom));
        StartPosition = FormStartPosition.CenterParent;
        hud.Dock = DockStyle.Fill;
        hud.DrawContent = DrawDialog;
        Controls.Add(hud);
        hud.SizeChanged += delegate { Arrange(); PlaceFields(); Redraw(); };
    }

    // Resizable windows move their panels, notes and fields here (see Frame).
    protected virtual void Arrange() { }

    protected void Frame(Control c, RectangleF frame) { frames[c] = frame; }

    protected override void OnHandleCreated(EventArgs e)
    {
        base.OnHandleCreated(e);
        Hud.DarkTitleBar(this);
    }

    // Fonts and options set after Field change a field's height, so place them again now.
    protected override void OnLoad(EventArgs e)
    {
        base.OnLoad(e);
        Arrange();
        PlaceFields();
    }

    protected void Redraw() { hud.Redraw(); }

    protected PanelPart Panel(string title, float x, float y, float w, float h)
    {
        var panel = new PanelPart { Title = title, Box = new RectangleF(x, y, w, h) };
        panels.Add(panel);
        return panel;
    }

    // A small letter-spaced label above a field.
    protected Note Caption(string text, float x, float y)
    {
        var note = new Note { Text = Lang.T(text).ToUpperInvariant(), Box = new RectangleF(x, y, 400, 15), Font = Hud.Label(12), Caption = true };
        notes.Add(note);
        return note;
    }

    // Wrapping text: help under a field, or a status message.
    protected Note Hint(string text, float x, float y, float w, float h)
    {
        var note = new Note { Text = Lang.T(text), Box = new RectangleF(x, y, w, h), Font = Hud.Body(13) };
        notes.Add(note);
        return note;
    }

    protected HudView.Item Button(string text, float x, float y, float w, MethodInvoker click)
    {
        var item = hud.Add(HudView.Kind.Button, text, click);
        item.Box = new RectangleF(x, y, w, 32);
        return item;
    }

    protected HudView.Item Primary(string text, float x, float y, float w, MethodInvoker click)
    {
        var item = Button(text, x, y, w, click);
        item.Kind = HudView.Kind.Primary;
        return item;
    }

    protected HudView.Item Switch(string text, bool on, float x, float y, float w)
    {
        var item = hud.Add(HudView.Kind.Switch, text, null);
        item.On = on;
        item.Box = new RectangleF(x, y, w, 24);
        return item;
    }

    // A real control in a drawn frame (x, y, w, h is the frame). Text boxes and lists lose
    // their own border, so the frame is the only one; the frame lights up while it has focus.
    protected T Field<T>(T c, float x, float y, float w, float h) where T : Control
    {
        c.BackColor = Hud.Back;
        c.ForeColor = Hud.Ink;
        var text = c as TextBoxBase;
        if (text != null) text.BorderStyle = BorderStyle.None;
        var list = c as ListBox;
        if (list != null) list.BorderStyle = BorderStyle.None;
        var view = c as ListView;
        if (view != null) view.BorderStyle = BorderStyle.None;
        var number = c as UpDownBase;
        if (number != null) number.BorderStyle = BorderStyle.None;
        if (list != null || view != null || (text != null && text.Multiline)) Hud.DarkScrollbars(c);
        frames[c] = new RectangleF(x, y, w, h);
        c.GotFocus += delegate { Redraw(); };
        c.LostFocus += delegate { Redraw(); };
        c.EnabledChanged += delegate { Redraw(); };
        fields.Add(c);
        hud.Controls.Add(c);
        PlaceFields();
        return c;
    }

    // Fields sit inside their frames: single-line ones centered, the rest filling it.
    void PlaceFields()
    {
        float z = hud.Zoom;
        foreach (var c in fields)
        {
            var f = frames[c];
            var text = c as TextBoxBase;
            bool oneLine = c is ComboBox || c is UpDownBase || (text != null && !text.Multiline);
            if (c is ComboBox)
                c.Bounds = new Rectangle((int)(f.X * z), (int)((f.Y + 1) * z), (int)(f.Width * z), (int)((f.Height - 2) * z));
            else if (oneLine)
            {
                int inner = Math.Max(c.Height, c.Font.Height); // single-line boxes size their own height
                c.Bounds = new Rectangle((int)((f.X + 7) * z), (int)(f.Y * z + (f.Height * z - inner) / 2), (int)((f.Width - 9) * z), inner);
            }
            else c.Bounds = new Rectangle((int)((f.X + 6) * z), (int)((f.Y + 5) * z), (int)((f.Width - 8) * z), (int)((f.Height - 9) * z));
        }
    }

    void DrawDialog(Graphics g)
    {
        foreach (var p in panels) { Hud.PanelBox(g, p.Box, p.Title); hud.Covers.Add(p.Box); }
        foreach (var n in notes) hud.Covers.Add(n.Box);
        foreach (var c in fields)
        {
            var f = frames[c];
            // A flat combo box draws its own border; it still gets the focus ring.
            if (!(c is ComboBox))
                using (var fill = new SolidBrush(Hud.Back)) g.FillRectangle(fill, f);
            if (c.ContainsFocus)
                using (var pen = new Pen(Hud.Accent, 2f)) g.DrawRectangle(pen, f.X - 1, f.Y - 1, f.Width + 2, f.Height + 2);
            else if (!(c is ComboBox))
                using (var pen = new Pen(c.Enabled ? Hud.Mix(Hud.Line, Hud.Dim, 0.25f) : Hud.Line, 1f)) g.DrawRectangle(pen, f.X, f.Y, f.Width, f.Height);
        }
        foreach (var n in notes)
        {
            if (n.Caption) Hud.Spaced(g, n.Text, n.Font, n.Color, n.Box.X, n.Box.Y, 1.2f);
            else if (n.Mark != Color.Empty)
            {
                Hud.Dot(g, n.Mark, n.Box.X + 4, n.Box.Y + 10);
                Hud.Wrapped(g, n.Text, n.Font, Hud.Ink, new RectangleF(n.Box.X + 16, n.Box.Y, n.Box.Width - 16, n.Box.Height));
            }
            else Hud.Wrapped(g, n.Text, n.Font, n.Color, n.Box);
        }
    }

    // Enter presses the default button and Esc the cancel one, unless a drawn item or a
    // field that takes them has the focus.
    protected override bool ProcessDialogKey(Keys keyData)
    {
        if (keyData == Keys.Enter && Accept != null) { Accept(); return true; }
        if (keyData == Keys.Escape && Cancel != null) { Cancel(); return true; }
        return base.ProcessDialogKey(keyData);
    }
}

class DeckWindow : Form
{
    const string RunKey = @"Software\Microsoft\Windows\CurrentVersion\Run";
    // The old name, so "Start with Windows" turned on in an older version stays on.
    const string RunValue = "Tablet Deck";
    // The window's size in 96-dpi pixels; it scales with the screen.
    const int DesignWidth = 900, DesignHeight = 688;

    readonly string appDir = Path.GetDirectoryName(Application.ExecutablePath);
    readonly string serverArgs;
    bool startHidden;
    bool quitting;
    bool trayHintShown;
    Process server;
    string link = "";
    string settingsPath = "";
    string fatalText = "";

    // What the panels show; DrawHud draws them from these.
    readonly HudView hud = new HudView();
    string[] qrRows = new string[0];
    // Status lines are kept as functions, so they show in the current language even after
    // it changes.
    string chip = "STARTING", others = "", pairingCode = "-", version = "";
    Func<string> stateText = () => Lang.T("Starting...");
    Color stateColor = Hud.Dim;
    bool dryRun, focusCheck = true;
    int tabletCount;
    DateTime runningSince = DateTime.MinValue;
    Func<string> voiceText = () => Lang.T("Starting..."), lastHeard = () => "";
    Color voiceColor = Hud.Dim, lastHeardColor = Hud.Dim;
    // Presses from the tablets and voice, for the telemetry panel.
    readonly Queue<DateTime> recentPresses = new Queue<DateTime>();
    int pressCount, blockedCount;
    string lastPress = "";
    // Panel boxes, set by Arrange.
    RectangleF connectBox, linkBox, telemetryBox, voiceBox, systemBox, logBox, bannerBox, networkBox;

    readonly HudView.Item addressLink, copyButton, settingsButton, editButton, restartButton;
    readonly HudView.Item voiceListButton, strictness, startWithWindows, autoUpdate, folderLink, checkLink, feedbackLink;
    // The Public-network banner's button: opens Windows' settings for that network.
    readonly HudView.Item networkButton;
    // The adapter Windows treats as a Public network (tablets can't connect), or "".
    string publicNetwork = "";
    bool publicNetworkNoticed;
    float networkGrown; // how much taller the window got for the banner, in 96-dpi pixels
    const string FeedbackPage = "https://github.com/xattribution/SpoolUp/issues/new/choose";
    readonly HudView.Item bannerWhatsNew, bannerUpdate, bannerLater;
    string editorUrl = ""; // the layout editor on this PC, set when the server is ready
    bool serverReady; // the server has said "ready" and takes settings on stdin
    Dictionary<string, object> readyGame; // the copy of the game in use, from "ready" (gameInUse in server.js)
    readonly RichTextBox log = new RichTextBox();
    readonly NotifyIcon tray = new NotifyIcon();
    readonly System.Windows.Forms.Timer copiedTimer = new System.Windows.Forms.Timer();
    // Sends the slider's value once it has stopped moving, not on every step of a drag.
    readonly System.Windows.Forms.Timer strictnessTimer = new System.Windows.Forms.Timer();
    // Once a second: the uptime and the presses graph.
    readonly System.Windows.Forms.Timer second = new System.Windows.Forms.Timer { Interval = 1000 };
    readonly List<string[]> voiceCommands = new List<string[]>(); // [phrase, button label]
    VoiceCommandsWindow commandsWindow;

    // ---- updates ----
    const string LatestReleaseApi = "https://api.github.com/repos/xattribution/SpoolUp/releases/latest";
    const string ReleasesPage = "https://github.com/xattribution/SpoolUp/releases";
    const string DownloadPrefix = "https://github.com/xattribution/SpoolUp/releases/download/";
    const string AppKey = @"Software\Tablet Deck"; // the old name, like Lang's key
    // This window's version, e.g. "1.1.0" (AssemblyInformationalVersion, set by scripts/build.js).
    static readonly string CurrentVersion = Application.ProductVersion;
    bool checkedAtStart;
    bool checking;
    readonly System.Windows.Forms.Timer dailyCheck = new System.Windows.Forms.Timer();
    bool bannerShown;
    Func<string> bannerText = () => "";
    Func<string> trayText = () => "SpoolUp";
    string whatsNewUrl = "";
    string updateVersion = "", updateSetupUrl = "", updateShaUrl = "", dismissedVersion = "";
    Process installer;

    public DeckWindow(string[] args)
    {
        var passOn = new List<string>();
        foreach (var arg in args)
        {
            if (arg == "--minimized") startHidden = true;
            else passOn.Add(arg.IndexOfAny(new[] { ' ', '"' }) >= 0 ? "\"" + arg.Replace("\"", "\\\"") + "\"" : arg);
        }
        serverArgs = ("--ui " + string.Join(" ", passOn)).Trim();

        SuspendLayout();
        Text = "SpoolUp";
        Icon = Icon.ExtractAssociatedIcon(Application.ExecutablePath);
        BackColor = Hud.Back;
        ForeColor = Hud.Ink;
        Font = new Font("Segoe UI", 9.75f);
        AutoScaleMode = AutoScaleMode.None; // HudView scales itself
        hud.Zoom = Hud.ScreenZoom();
        ClientSize = new Size((int)(DesignWidth * hud.Zoom), (int)(DesignHeight * hud.Zoom));
        MinimumSize = SizeFromClientSize(ClientSize);
        StartPosition = FormStartPosition.CenterScreen;

        hud.Dock = DockStyle.Fill;
        hud.DrawContent = DrawHud;
        Controls.Add(hud);

        // First, so Tab reaches it first while the banner shows.
        networkButton = hud.Add(HudView.Kind.Primary, "Open network settings", delegate { OpenNetworkSettings(publicNetwork); });
        networkButton.Look = HudView.OnWarn;
        networkButton.Visible = false;
        networkButton.Tip = "Pick your network there and set its network profile to Private";

        addressLink = hud.Add(HudView.Kind.Link, "", delegate { if (link != "") Process.Start(link); });
        addressLink.Mono = true;
        addressLink.Tip = "Opens the deck in this PC's browser";
        copyButton = hud.Add(HudView.Kind.Button, "Copy link", delegate { CopyLink(); });
        copyButton.Tip = "Copies the address with the pairing code, to send to a tablet";
        settingsButton = hud.Add(HudView.Kind.Button, "Settings", delegate { OpenSettings(); });
        settingsButton.Tip = "Pairing code, port, focus check, tap length, voice language and the Star Citizen keybind profile";
        // Edit layout and Restart share a place: one shows while running, the other when stopped.
        editButton = hud.Add(HudView.Kind.Primary, "Edit layout", delegate { OpenEditor(); });
        editButton.Visible = false;
        editButton.Tip = "Opens the layout editor in your browser: move buttons, change their keys, labels and icons";
        restartButton = hud.Add(HudView.Kind.Primary, "Restart", delegate { StartServer(); });
        restartButton.Visible = false;

        voiceListButton = hud.Add(HudView.Kind.Button, "Commands", delegate { ShowVoiceCommands(); });
        voiceListButton.Enabled = false;
        voiceListButton.Tip = "Everything you can say, and the button each one presses";
        // How sure Windows' speech recognition must be before a command counts (voiceConfidence).
        strictness = hud.Add(HudView.Kind.Slider, "", null);
        strictness.Min = 20;
        strictness.Max = 95;
        strictness.Value = 60;
        strictness.Enabled = false;
        strictness.Changed = delegate { strictnessTimer.Stop(); strictnessTimer.Start(); };
        strictness.Tip = "Lower accepts more but may mishear. Higher is stricter.";

        startWithWindows = hud.Add(HudView.Kind.Switch, "Start with Windows (in the tray)", null);
        startWithWindows.On = RunsAtStartup();
        startWithWindows.Click = delegate { SetRunAtStartup(startWithWindows.On); };
        autoUpdate = hud.Add(HudView.Kind.Switch, "Check for updates automatically", null);
        autoUpdate.On = true;
        autoUpdate.Click = delegate
        {
            SendToServer("set autoUpdate " + (autoUpdate.On ? "on" : "off"));
            if (autoUpdate.On) CheckForUpdates(false);
        };
        autoUpdate.Tip = "Looks for a new version on GitHub when SpoolUp starts and once a day. Click Update now to install it.";
        folderLink = hud.Add(HudView.Kind.Link, "Settings folder", delegate { OpenSettingsFolder(); });
        folderLink.Tip = "Opens the folder with config.json and your own layouts";
        checkLink = hud.Add(HudView.Kind.Link, "Check for updates now", delegate { CheckForUpdates(true); });
        feedbackLink = hud.Add(HudView.Kind.Link, "Feedback & ideas", delegate { Process.Start(FeedbackPage); });
        feedbackLink.Tip = "Report a problem or suggest an idea on GitHub";

        // The update banner, above the panels when there's news.
        bannerWhatsNew = hud.Add(HudView.Kind.Button, "What's new", delegate { if (whatsNewUrl != "") Process.Start(whatsNewUrl); });
        bannerUpdate = hud.Add(HudView.Kind.Primary, "Update now", delegate { InstallUpdate(); });
        bannerLater = hud.Add(HudView.Kind.Button, "Later", delegate { dismissedVersion = updateVersion; HideBanner(); });
        foreach (var b in new[] { bannerWhatsNew, bannerUpdate, bannerLater }) b.Visible = false;

        strictnessTimer.Interval = 400;
        strictnessTimer.Tick += delegate
        {
            strictnessTimer.Stop();
            SendToServer("set voiceConfidence " + (strictness.Value / 100.0).ToString("0.00", CultureInfo.InvariantCulture));
        };

        log.ReadOnly = true;
        log.BackColor = Hud.Back;
        log.ForeColor = Hud.Ink;
        log.BorderStyle = BorderStyle.None;
        log.Font = new Font("Consolas", 10f);
        log.DetectUrls = false;
        log.WordWrap = true;
        log.ScrollBars = RichTextBoxScrollBars.Vertical;
        Hud.DarkScrollbars(log);
        hud.Controls.Add(log);

        hud.SizeChanged += delegate { Arrange(); };
        Arrange();

        second.Tick += delegate
        {
            while (recentPresses.Count > 0 && (DateTime.Now - recentPresses.Peek()).TotalSeconds > 60) recentPresses.Dequeue();
            if (Visible && WindowState != FormWindowState.Minimized) hud.Redraw();
        };
        second.Start();

        dailyCheck.Interval = 24 * 60 * 60 * 1000;
        dailyCheck.Tick += delegate { if (autoUpdate.On) CheckForUpdates(false); };
        dailyCheck.Start();

        tray.Icon = Icon;
        tray.Text = trayText();
        tray.Visible = true;
        tray.DoubleClick += delegate { ShowFromTray(); };
        BuildTrayMenu();

        copiedTimer.Interval = 1500;
        copiedTimer.Tick += delegate { copiedTimer.Stop(); copyButton.Text = "Copy link"; hud.Redraw(); };

        ResumeLayout(false);

        if (!IsHandleCreated) CreateHandle(); // lets server output reach the window even while it starts hidden
        StartServer();
        ShowWhatsNewAfterUpdate();
    }

    protected override void OnHandleCreated(EventArgs e)
    {
        base.OnHandleCreated(e);
        Hud.DarkTitleBar(this);
    }

    // Windows' settings page for the network an adapter is on: the Wi-Fi or Ethernet page when
    // the adapter's name says which, where "Network profile type" sits near the top, or the
    // general network status page. Shared with Settings.
    public static void OpenNetworkSettings(string adapter)
    {
        string name = (adapter ?? "").ToLowerInvariant();
        string page = name.Contains("wi-fi") || name.Contains("wifi") || name.Contains("wlan") || name.Contains("wireless") ? "ms-settings:network-wifi"
            : name.Contains("ethernet") ? "ms-settings:network-ethernet" : "ms-settings:network-status";
        try { Process.Start(page); }
        catch (Exception)
        {
            try { Process.Start("ms-settings:network-status"); } catch (Exception) { }
        }
    }

    // The stars only move while this window is in front.
    protected override void OnActivated(EventArgs e) { base.OnActivated(e); hud.Running = true; }
    protected override void OnDeactivate(EventArgs e) { base.OnDeactivate(e); hud.Running = false; }

    // ---------- layout and drawing ----------

    // Places the panels and items for the window's size, in 96-dpi pixels: the header, the
    // Public-network and update banners if shown, CONNECT and LINK side by side, TELEMETRY
    // under LINK, VOICE and SYSTEM below, and the log filling the rest. 16 pixels around the
    // edges, 12 between panels.
    void Arrange()
    {
        var area = hud.Area;
        if (area.Width < 200 || area.Height < 200) return; // minimized
        float w = area.Width, top = 76;
        hud.StarBand = new RectangleF(0, 0, w, 64);
        if (publicNetwork != "")
        {
            networkBox = new RectangleF(16, top, w - 32, NetworkBannerHeight(w - 32));
            top += networkBox.Height + 12;
        }
        bannerBox = new RectangleF(16, top, w - 32, 44);
        if (bannerShown) top += 56;
        connectBox = new RectangleF(16, top, 280, 312);
        linkBox = new RectangleF(308, top, w - 324, 176);
        telemetryBox = new RectangleF(308, top + 188, w - 324, 124);
        voiceBox = new RectangleF(16, top + 324, w - 360, 152);
        systemBox = new RectangleF(w - 332, top + 324, 316, 152);
        logBox = new RectangleF(16, top + 488, w - 32, Math.Max(84, area.Height - top - 488 - 16));

        float x = linkBox.X + 16, right = linkBox.Right - 16, row = linkBox.Bottom - 48;
        addressLink.Box = new RectangleF(x, linkBox.Y + 58, right - x, 26);
        editButton.Box = restartButton.Box = new RectangleF(right - 136, row, 136, 32);
        settingsButton.Box = new RectangleF(right - 136 - 8 - 112, row, 112, 32);
        copyButton.Box = new RectangleF(right - 136 - 8 - 112 - 8 - 112, row, 112, 32);

        voiceListButton.Box = new RectangleF(voiceBox.Right - 16 - 120, voiceBox.Y + 36, 120, 32);
        strictness.Box = new RectangleF(voiceBox.X + 132, voiceBox.Y + 82, 200, 18);
        startWithWindows.Box = new RectangleF(systemBox.X + 16, systemBox.Y + 40, systemBox.Width - 32, 24);
        autoUpdate.Box = new RectangleF(systemBox.X + 16, systemBox.Y + 72, systemBox.Width - 32, 24);
        folderLink.Box = new RectangleF(systemBox.X + 16, systemBox.Y + 106, 124, 20);
        feedbackLink.Box = new RectangleF(systemBox.X + 148, systemBox.Y + 106, systemBox.Width - 164, 20);
        checkLink.Box = new RectangleF(systemBox.X + 16, systemBox.Y + 128, systemBox.Width - 32, 20);

        if (publicNetwork != "")
            networkButton.Box = new RectangleF(networkBox.Right - 16 - 232, networkBox.Y + (networkBox.Height - 36) / 2, 232, 36);
        float bx = bannerBox.Right - 6;
        foreach (var b in new[] { bannerLater, bannerUpdate, bannerWhatsNew })
        {
            float bw = b == bannerUpdate ? 128 : 108;
            if (b.Visible) { bx -= bw; b.Box = new RectangleF(bx, bannerBox.Y + 6, bw, 32); bx -= 8; }
        }

        float z = hud.Zoom;
        log.Bounds = new Rectangle((int)((logBox.X + 22) * z), (int)((logBox.Y + 46) * z), (int)((logBox.Width - 44) * z), (int)((logBox.Height - 58) * z));
        hud.Redraw();
    }

    // The Public-network banner's text, and the height it needs at a width.
    string NetworkBody() { return Lang.F("Windows treats \"{0}\" as a Public network. Set it to Private so tablets on your network can reach SpoolUp.", publicNetwork); }
    static string NetworkSteps() { return Lang.T("Choose your network, then Private network. SpoolUp checks again every 20 seconds."); }
    static float NetworkTextWidth(float width) { return width - 68 - 16 - 232 - 16; }

    float NetworkBannerHeight(float width)
    {
        float tw = NetworkTextWidth(width);
        float h = 12 + 26 + Hud.WrappedHeight(NetworkBody(), Hud.Body(13), tw) + 4 + Hud.WrappedHeight(NetworkSteps(), Hud.Body(13), tw) + 14;
        return Math.Max(88, h);
    }

    void DrawHud(Graphics g)
    {
        var area = hud.Area;
        hud.Covers.AddRange(new[] { connectBox, linkBox, telemetryBox, voiceBox, systemBox, logBox });
        DrawHeader(g, area.Width);
        if (publicNetwork != "") DrawNetworkBanner(g);
        if (bannerShown)
        {
            Hud.Box(g, bannerBox, 6, Hud.Mix(Hud.Panel, Hud.Accent, 0.14f), Hud.Mix(Hud.Line, Hud.Accent, 0.7f));
            float buttonsLeft = bannerBox.Right;
            foreach (var b in new[] { bannerWhatsNew, bannerUpdate, bannerLater }) if (b.Visible) buttonsLeft = Math.Min(buttonsLeft, b.Box.X);
            Hud.Dot(g, Hud.Accent, bannerBox.X + 20, bannerBox.Y + bannerBox.Height / 2);
            Hud.Text(g, bannerText(), Hud.Body(13.5f), Hud.Ink, new RectangleF(bannerBox.X + 34, bannerBox.Y, buttonsLeft - bannerBox.X - 46, bannerBox.Height));
        }
        DrawConnect(g);
        DrawLink(g);
        DrawTelemetry(g);
        DrawVoice(g);
        Hud.PanelBox(g, systemBox, "SYSTEM");
        Hud.PanelBox(g, logBox, "LOG");
        Hud.Box(g, new RectangleF(logBox.X + 12, logBox.Y + 40, logBox.Width - 24, logBox.Height - 52), 4, Hud.Back, Hud.Line);
    }

    // Solid amber across the window, dark text, and the one button that fixes it.
    void DrawNetworkBanner(Graphics g)
    {
        var r = networkBox;
        Hud.Box(g, r, 6, Hud.Warn, Color.Empty);
        float gx = r.X + 20, gy = r.Y + 16;
        using (var dark = new SolidBrush(Hud.WarnInk))
            g.FillPolygon(dark, new[] { new PointF(gx + 16, gy), new PointF(gx + 33, gy + 29), new PointF(gx - 1, gy + 29) });
        var bang = Hud.Head(17);
        Hud.Spaced(g, "!", bang, Hud.Warn, gx + 16 - Hud.Width(g, "!", bang) / 2, gy + 9, 0);
        float tx = r.X + 68, tw = NetworkTextWidth(r.Width);
        Hud.Text(g, Lang.T("Tablets can't connect"), Hud.Head(16), Hud.WarnInk, new RectangleF(tx, r.Y + 12, tw, 24));
        float bh = Hud.WrappedHeight(NetworkBody(), Hud.Body(13), tw);
        Hud.Wrapped(g, NetworkBody(), Hud.Body(13), Hud.WarnInk, new RectangleF(tx, r.Y + 38, tw, bh + 2));
        Hud.Wrapped(g, NetworkSteps(), Hud.Body(13), Hud.WarnInk, new RectangleF(tx, r.Y + 42 + bh, tw, Math.Max(18, r.Bottom - (r.Y + 42 + bh))));
    }

    void DrawHeader(Graphics g, float width)
    {
        // An original mark: a hexagon frame around a pointer, like a cockpit indicator.
        var hex = new PointF[6];
        for (int i = 0; i < 6; i++) hex[i] = new PointF(32 + 15 * (float)Math.Cos(Math.PI / 3 * i + Math.PI / 6), 32 + 15 * (float)Math.Sin(Math.PI / 3 * i + Math.PI / 6));
        using (var pen = new Pen(Hud.Accent, 2f)) g.DrawPolygon(pen, hex);
        using (var brush = new SolidBrush(Hud.Accent))
            g.FillPolygon(brush, new[] { new PointF(32, 23), new PointF(38, 38), new PointF(32, 34), new PointF(26, 38) });
        float tw = Hud.Spaced(g, "SPOOLUP", Hud.Head(24), Hud.Ink, 58, 12, 3f), vw = 0;
        if (version != "") vw = 12 + Hud.Spaced(g, "V" + version, Hud.Label(12), Hud.Dim, 58 + tw + 12, 22, 1.2f);
        float sw = Hud.Spaced(g, Lang.T("STAR CITIZEN CONTROL DECK"), Hud.Label(12), Hud.Dim, 59, 41, 2f);
        hud.Covers.Add(new RectangleF(12, 6, Math.Max(58 + tw + vw, 59 + sw) + 8, 54));

        // Status on the right: a pill with a dot and one word, the uptime under it.
        var pill = Hud.Pill(g, Lang.T(chip), stateColor, width - 16, 9);
        hud.Covers.Add(RectangleF.Inflate(pill, 6, 4));
        if (runningSince != DateTime.MinValue)
        {
            var up = DateTime.Now - runningSince;
            string text = Lang.T("UPTIME") + " " + ((int)up.TotalHours).ToString("00") + ":" + up.Minutes.ToString("00") + ":" + up.Seconds.ToString("00");
            float uw = Hud.SpacedWidth(g, text, Hud.Mono(12, false), 0.6f);
            Hud.Spaced(g, text, Hud.Mono(12, false), Hud.Dim, width - 18 - uw, 40, 0.6f);
            hud.Covers.Add(new RectangleF(width - 26 - uw, 36, uw + 16, 22));
        }
        using (var pen = new Pen(Hud.Line, 1f)) g.DrawLine(pen, 0, 64, width, 64);
    }

    void DrawConnect(Graphics g)
    {
        var r = connectBox;
        Hud.PanelBox(g, r, "CONNECT");
        var qrBox = new RectangleF(r.X + 22, r.Y + 40, 236, 236);
        if (qrRows.Length > 0) hud.DrawQr(g, qrBox, qrRows);
        else
        {
            Hud.Box(g, qrBox, 4, Hud.Back, Hud.Line);
            string text = Lang.T(serverReady ? "NO NETWORK FOUND" : chip == "STOPPED" ? "OFFLINE" : "WAITING FOR THE SERVER");
            float w = Hud.SpacedWidth(g, text, Hud.Label(12), 1.2f);
            Hud.Spaced(g, text, Hud.Label(12), Hud.Dim, qrBox.X + (qrBox.Width - w) / 2, qrBox.Y + qrBox.Height / 2 - 7, 1.2f);
        }
        string caption = Lang.T("SCAN WITH YOUR TABLET'S CAMERA");
        float cw = Hud.SpacedWidth(g, caption, Hud.Label(12), 1.2f);
        Hud.Spaced(g, caption, Hud.Label(12), Hud.Dim, r.X + (r.Width - cw) / 2, r.Bottom - 26, 1.2f);
    }

    void DrawLink(Graphics g)
    {
        var r = linkBox;
        float x = r.X + 16, w = r.Width - 32;
        Hud.PanelBox(g, r, "LINK");
        if (serverReady)
        {
            Hud.Spaced(g, Lang.T("ADDRESS"), Hud.Label(12), Hud.Dim, x, r.Y + 40, 1.2f);
            if (link == "")
            {
                Hud.Dot(g, Hud.Warn, x + 4, r.Y + 71);
                Hud.Text(g, Lang.T("No network found. Connect this PC to Wi-Fi or a network."), Hud.Body(13), Hud.Ink, new RectangleF(x + 16, r.Y + 60, w - 16, 22));
            }
            if (others != "") Hud.Text(g, Lang.T("ALSO") + "  " + others, Hud.Mono(12, false), Hud.Dim, new RectangleF(x, r.Y + 88, w, 18));
        }
        else
        {
            Hud.Dot(g, stateColor, x + 4, r.Y + 50);
            Hud.Wrapped(g, stateText(), Hud.Body(13.5f), Hud.Ink, new RectangleF(x + 16, r.Y + 40, w - 16, 66));
        }
        Hud.Spaced(g, Lang.T("PAIRING CODE"), Hud.Label(12), Hud.Dim, x, r.Y + 112, 1.2f);
        Hud.Spaced(g, pairingCode, Hud.Mono(28, true), Hud.Ink, x - 1, r.Y + 128, 3f);
    }

    void DrawTelemetry(Graphics g)
    {
        var r = telemetryBox;
        Hud.PanelBox(g, r, "TELEMETRY");
        float y = r.Y + 40, h = r.Height - 52, x = r.X + 16, gap = 10;
        float total = r.Width - 32 - gap * 2;
        var tablets = new RectangleF(x, y, total * 0.24f, h);
        var keys = new RectangleF(tablets.Right + gap, y, total * 0.31f, h);
        var presses = new RectangleF(keys.Right + gap, y, r.Right - 16 - keys.Right - gap, h);

        Tile(g, tablets, "TABLETS", tabletCount.ToString(), Color.Empty, "CONNECTED");
        string target = !serverReady ? "-" : dryRun ? "LOG ONLY" : focusCheck ? "STAR CITIZEN" : "ANY WINDOW";
        string why = !serverReady ? "" : dryRun ? "TEST MODE: NO KEYS SENT" : focusCheck ? "ONLY WHILE IT'S IN FRONT" : "FOCUS CHECK IS OFF";
        Tile(g, keys, "KEYS GO TO", target, !serverReady ? Color.Empty : dryRun || !focusCheck ? Hud.Warn : Hud.Good, why);

        Tile(g, presses, "PRESSES", pressCount.ToString(), Color.Empty, lastPress != "" ? Lang.F("LAST: {0}", lastPress.ToUpperInvariant()) : "NONE YET");
        float graphTop = 26;
        if (blockedCount > 0)
        {
            // Beside the caption when there's room, else in a line of its own under the graph.
            string text = Lang.F("{0} BLOCKED", blockedCount);
            float bw = Hud.SpacedWidth(g, text, Hud.Label(12), 1f);
            float caption = Hud.SpacedWidth(g, Lang.T("PRESSES"), Hud.Label(12), 1.2f);
            float by = presses.Width - 20 - caption - 24 > bw ? presses.Y + 8 : presses.Y + 25;
            if (by > presses.Y + 8) graphTop = 44;
            Hud.Spaced(g, text, Hud.Label(12), Hud.Ink, presses.Right - 10 - bw, by, 1f);
            Hud.Dot(g, Hud.Warn, presses.Right - 20 - bw, by + 7);
        }
        // The last minute in 2-second bars.
        var graph = new RectangleF(presses.X + presses.Width * 0.45f, presses.Y + graphTop, presses.Width * 0.55f - 12, presses.Height - graphTop - 8);
        var bars = new int[30];
        var now = DateTime.Now;
        foreach (var t in recentPresses)
        {
            int i = 29 - (int)((now - t).TotalSeconds / 2);
            if (i >= 0 && i < 30) bars[i]++;
        }
        int most = 3;
        foreach (int b in bars) most = Math.Max(most, b);
        float bar = graph.Width / 30;
        using (var brush = new SolidBrush(Hud.Accent))
            for (int i = 0; i < 30; i++)
                if (bars[i] > 0)
                {
                    float bh = Math.Max(2, graph.Height * bars[i] / most);
                    g.FillRectangle(brush, graph.X + i * bar + 1, graph.Bottom - bh, bar - 2, bh);
                }
        using (var pen = new Pen(Hud.Line, 1f)) g.DrawLine(pen, graph.X, graph.Bottom + 1, graph.Right, graph.Bottom + 1);
    }

    // A readout: caption, value and a note, with a status dot by the caption when mark is set.
    static void Tile(Graphics g, RectangleF r, string caption, string value, Color mark, string note)
    {
        Hud.Box(g, r, 4, Hud.Back, Hud.Line);
        float cw = Hud.Spaced(g, Lang.T(caption), Hud.Label(12), Hud.Dim, r.X + 10, r.Y + 8, 1.2f);
        if (mark != Color.Empty) Hud.Dot(g, mark, r.X + 20 + cw, r.Y + 15);
        value = Lang.T(value);
        var font = Hud.Head(value.Length > 6 ? 16 : 22);
        Hud.Text(g, value, font, Hud.Ink, new RectangleF(r.X + 10, r.Y + 23, r.Width - 16, 28));
        Hud.Text(g, Lang.T(note), Hud.Label(11.5f), Hud.Dim, new RectangleF(r.X + 10, r.Bottom - 19, r.Width - 16, 15));
    }

    void DrawVoice(Graphics g)
    {
        var r = voiceBox;
        float x = r.X + 16;
        Hud.PanelBox(g, r, "VOICE");
        Hud.Dot(g, voiceColor, x + 4, r.Y + 52);
        Hud.Text(g, voiceText(), Hud.Body(13), Hud.Ink, new RectangleF(x + 16, r.Y + 40, voiceListButton.Box.X - x - 28, 24));
        Hud.Spaced(g, Lang.T("STRICTNESS"), Hud.Label(12), Hud.Dim, x, r.Y + 84, 1.2f);
        Hud.Spaced(g, strictness.Value + "%", Hud.Head(13), strictness.Enabled ? Hud.Ink : Hud.Dim, strictness.Box.Right + 14, r.Y + 83, 0.6f);
        Hud.Text(g, Lang.T("Lower accepts more but may mishear. Higher is stricter."), Hud.Body(13), Hud.Dim, new RectangleF(x, r.Y + 104, r.Width - 32, 20));
        string heard = lastHeard();
        if (heard != "")
        {
            Hud.Dot(g, lastHeardColor, x + 4, r.Y + 137);
            Hud.Text(g, heard, Hud.Body(13), lastHeardColor == Hud.Dim ? Hud.Dim : Hud.Ink, new RectangleF(x + 16, r.Y + 126, r.Width - 48, 22));
        }
    }

    // ---------- server ----------

    void StartServer()
    {
        serverReady = false;
        fatalText = "";
        restartButton.Visible = false;
        copyButton.Enabled = false;
        link = "";
        addressLink.Visible = false;
        others = "";
        pairingCode = "-";
        qrRows = new string[0];
        lastHeard = () => "";
        strictness.Enabled = false;
        voiceCommands.Clear();
        voiceListButton.Enabled = false;
        if (commandsWindow != null) commandsWindow.Fill();
        SetState("STARTING", () => Lang.T("Starting the server..."), Hud.Dim);
        voiceText = () => Lang.T("Starting...");
        voiceColor = Hud.Dim;
        tabletCount = 0;
        string exe = Path.Combine(appDir, "TabletDeckServer.exe");
        if (!File.Exists(exe))
        {
            Stopped(() => Lang.F("{0} is missing. Reinstall SpoolUp.", exe));
            return;
        }
        var psi = new ProcessStartInfo(exe, serverArgs)
        {
            UseShellExecute = false,
            CreateNoWindow = true,
            WorkingDirectory = appDir,
            RedirectStandardInput = true,
            RedirectStandardOutput = true,
            RedirectStandardError = true,
            StandardOutputEncoding = Encoding.UTF8,
            StandardErrorEncoding = Encoding.UTF8,
        };
        var proc = new Process { StartInfo = psi, EnableRaisingEvents = true };
        proc.OutputDataReceived += (s, e) => { if (e.Data != null) OnUi(delegate { OnLine(e.Data, false); }); };
        proc.ErrorDataReceived += (s, e) => { if (e.Data != null) OnUi(delegate { OnLine(e.Data, true); }); };
        proc.Exited += delegate { OnUi(delegate { OnServerExit(proc); }); };
        try
        {
            proc.Start();
        }
        catch (Exception ex)
        {
            Stopped(() => Lang.F("Couldn't start the server: {0}", ex.Message));
            return;
        }
        server = proc;
        proc.BeginOutputReadLine();
        proc.BeginErrorReadLine();
    }

    void OnUi(MethodInvoker action)
    {
        if (IsDisposed || !IsHandleCreated) return;
        try { BeginInvoke(action); } catch (InvalidOperationException) { }
    }

    // Asks the server to release held keys and stop; ends it if it doesn't within 3 s.
    void StopServer()
    {
        var proc = server;
        server = null;
        if (proc == null || proc.HasExited) return;
        try
        {
            proc.StandardInput.WriteLine("quit");
            proc.StandardInput.Flush();
        }
        catch (Exception) { }
        if (!proc.WaitForExit(3000))
        {
            try { proc.Kill(); } catch (Exception) { }
        }
    }

    void SendToServer(string line)
    {
        var proc = server;
        if (proc == null || proc.HasExited) return;
        try
        {
            proc.StandardInput.WriteLine(line);
            proc.StandardInput.Flush();
        }
        catch (Exception) { }
    }

    void OnServerExit(Process proc)
    {
        if (quitting || proc != server) return;
        server = null;
        string fatal = fatalText;
        int code = proc.ExitCode;
        Stopped(() => fatal != "" ? fatal : Lang.F("The server stopped (exit code {0}). See the log below.", code));
        ShowFromTray();
    }

    void Stopped(Func<string> why)
    {
        serverReady = false;
        // The address means nothing now; the reason takes its place.
        link = "";
        addressLink.Visible = false;
        copyButton.Enabled = false;
        others = "";
        qrRows = new string[0];
        runningSince = DateTime.MinValue;
        SetState("STOPPED", () => Lang.F("Stopped: {0}", why()), Hud.Bad);
        restartButton.Visible = true;
        editButton.Visible = false;
        tabletCount = 0;
        voiceText = () => Lang.T("Off");
        voiceColor = Hud.Dim;
        ShowTrayText(() => Lang.T("SpoolUp (stopped)"));
    }

    void OnLine(string line, bool error)
    {
        if (line.StartsWith("@ui "))
        {
            Dictionary<string, object> ev = null;
            try { ev = new JavaScriptSerializer().DeserializeObject(line.Substring(4)) as Dictionary<string, object>; }
            catch (Exception) { }
            if (ev != null) OnEvent(ev);
            return;
        }
        if (line.Trim() == "") return;
        AppendLog(line, error ? Hud.Warn : Hud.Ink);
    }

    void OnEvent(Dictionary<string, object> ev)
    {
        switch (Str(ev, "t"))
        {
            case "ready":
                serverReady = true;
                link = Str(ev, "link");
                editorUrl = "http://localhost:" + Str(ev, "port") + "/editor.html?t=" + Uri.EscapeDataString(Str(ev, "token"));
                editButton.Visible = true;
                settingsPath = Str(ev, "settings");
                readyGame = ev.ContainsKey("game") ? ev["game"] as Dictionary<string, object> : null;
                addressLink.Text = link;
                addressLink.Visible = copyButton.Enabled = link != "";
                pairingCode = Str(ev, "token");
                var otherLinks = new List<string>();
                foreach (var o in List(ev, "others")) otherLinks.Add(Convert.ToString(o, CultureInfo.InvariantCulture));
                others = string.Join("  ", otherLinks.ToArray());
                var rows = new List<string>();
                foreach (var r in List(ev, "qr")) rows.Add(Convert.ToString(r, CultureInfo.InvariantCulture));
                qrRows = rows.ToArray();
                version = Str(ev, "version");
                Text = "SpoolUp " + version;
                dryRun = Bool(ev, "dryRun");
                focusCheck = Bool(ev, "focusCheck");
                // A new pairing code or port restarts nothing, so the uptime keeps counting.
                if (runningSince == DateTime.MinValue) runningSince = DateTime.Now;
                string running = dryRun ? "Running in test mode (dry run): no keys are sent" : "Running";
                SetState(dryRun ? "TEST MODE" : "ONLINE", () => Lang.T(running), dryRun ? Hud.Warn : Hud.Good);
                string trayVersion = version;
                bool trayTest = dryRun;
                ShowTrayText(() => "SpoolUp " + trayVersion + (trayTest ? Lang.T(" (test mode)") : ""));
                ShowStrictness(Num(ev, "voiceConfidence"));
                autoUpdate.On = Bool(ev, "autoUpdate");
                if (!checkedAtStart && autoUpdate.On) CheckForUpdates(false);
                checkedAtStart = true;
                break;
            case "voiceConfidence":
                ShowStrictness(Num(ev, "value"));
                break;
            case "voice-commands":
                voiceCommands.Clear();
                foreach (var pair in List(ev, "commands"))
                {
                    var p = pair as object[];
                    if (p != null && p.Length == 2) voiceCommands.Add(new[] { Convert.ToString(p[0], CultureInfo.InvariantCulture), Convert.ToString(p[1], CultureInfo.InvariantCulture) });
                }
                voiceListButton.Enabled = voiceCommands.Count > 0;
                if (commandsWindow != null) commandsWindow.Fill();
                break;
            case "heard":
                string result = Str(ev, "result"), heard = Str(ev, "text"), pressed = Str(ev, "label");
                int sure = (int)Math.Round(Num(ev, "confidence") * 100), below = strictness.Value;
                if (result == "accepted")
                {
                    lastHeard = () => Lang.F("Last heard: \"{0}\", {1}% sure. Pressed {2}.", heard, sure, pressed);
                    lastHeardColor = Hud.Good;
                }
                else if (result == "ignored")
                {
                    lastHeard = () => Lang.F("Last heard: \"{0}\", {1}% sure. Below {2}%, so it was ignored.", heard, sure, below);
                    lastHeardColor = Hud.Warn;
                }
                else
                {
                    lastHeard = () => Lang.T("Last heard: something that isn't one of the commands.");
                    lastHeardColor = Hud.Dim;
                }
                break;
            case "clients":
                int count;
                if (int.TryParse(Str(ev, "count"), out count)) tabletCount = count;
                break;
            case "press":
                if (Bool(ev, "blocked")) blockedCount++;
                else
                {
                    pressCount++;
                    recentPresses.Enqueue(DateTime.Now);
                    lastPress = Str(ev, "label");
                }
                break;
            case "voice":
                string s = Str(ev, "state"), phrases = Str(ev, "phrases"), language = Str(ev, "language"), ptt = Str(ev, "ptt"), why = Str(ev, "text");
                if (s == "ready")
                {
                    voiceText = () => Lang.F("Ready: {0} phrases, {1}", phrases, language) + (ptt != "" ? Lang.F(", push-to-talk: {0}", ptt) : "");
                    voiceColor = Hud.Good;
                }
                else if (s == "off")
                {
                    voiceText = () => Lang.T("Turned off in settings");
                    voiceColor = Hud.Dim;
                }
                else if (s == "starting")
                {
                    voiceText = () => Lang.T("Restarting...");
                    voiceColor = Hud.Dim;
                }
                else
                {
                    voiceText = () => Lang.F("Unavailable: {0}", why);
                    voiceColor = Hud.Warn;
                }
                break;
            case "network":
                SetPublicNetwork(Str(ev, "category") == "Public" ? Str(ev, "adapter") : "");
                if (publicNetwork != "" && !publicNetworkNoticed)
                {
                    publicNetworkNoticed = true;
                    AppendLog(Lang.F("Windows treats \"{0}\" as a Public network, so tablets can't connect. In Windows: Settings > Network & Internet > {0} > set it to Private.", publicNetwork), Hud.Warn);
                    if (!Visible) Notify(Lang.T("Tablets can't connect"), Lang.F("Windows treats \"{0}\" as a Public network. Open SpoolUp to fix it.", publicNetwork));
                }
                break;
            case "fatal":
                fatalText = Str(ev, "text");
                break;
        }
        hud.Redraw();
    }

    static string Str(Dictionary<string, object> d, string key)
    {
        object v;
        return d.TryGetValue(key, out v) && v != null ? Convert.ToString(v, CultureInfo.InvariantCulture) : "";
    }

    static bool Bool(Dictionary<string, object> d, string key)
    {
        object v;
        return d.TryGetValue(key, out v) && v is bool && (bool)v;
    }

    static double Num(Dictionary<string, object> d, string key)
    {
        object v;
        if (!d.TryGetValue(key, out v) || v == null) return 0;
        try { return Convert.ToDouble(v, CultureInfo.InvariantCulture); }
        catch (Exception) { return 0; }
    }

    static object[] List(Dictionary<string, object> d, string key)
    {
        object v;
        return d.TryGetValue(key, out v) && v is object[] ? (object[])v : new object[0];
    }

    // Moves the slider to the server's value without sending it back.
    void ShowStrictness(double value)
    {
        int percent = Math.Max(strictness.Min, Math.Min(strictness.Max, (int)Math.Round(value * 100)));
        strictness.Value = percent;
        strictness.Enabled = true;
        hud.Redraw();
    }

    void ShowVoiceCommands()
    {
        if (commandsWindow == null)
        {
            commandsWindow = new VoiceCommandsWindow(voiceCommands, Icon);
            commandsWindow.FormClosed += delegate { commandsWindow = null; };
            commandsWindow.Fill();
            commandsWindow.Show(this);
        }
        else commandsWindow.Activate();
    }

    // Shows the Public-network banner while adapter isn't "", and takes it away after. The
    // window grows to make room, as it does for the update banner.
    void SetPublicNetwork(string adapter)
    {
        if (adapter == publicNetwork) return;
        publicNetwork = adapter;
        networkButton.Visible = adapter != "";
        float want = adapter != "" ? NetworkBannerHeight(hud.Area.Width - 32) + 12 : 0;
        if (WindowState == FormWindowState.Normal)
        {
            Height = Math.Max(MinimumSize.Height, Height + (int)((want - networkGrown) * hud.Zoom));
            networkGrown = want;
        }
        Arrange();
    }

    // The header's one-word status, and the sentence shown while the server isn't running.
    void SetState(string word, Func<string> text, Color color)
    {
        chip = word;
        stateText = text;
        stateColor = color;
        hud.Redraw();
    }

    // Warnings get a "!" in the warning color and keep their text in Ink, like the rest.
    void AppendLog(string line, Color color)
    {
        // Keep the last 500 lines.
        if (log.Lines.Length > 500)
        {
            log.ReadOnly = false;
            log.Select(0, log.GetFirstCharIndexFromLine(100));
            log.SelectedText = "";
            log.ReadOnly = true;
        }
        log.SelectionStart = log.TextLength;
        log.SelectionLength = 0;
        if (color == Hud.Warn)
        {
            log.SelectionColor = Hud.Warn;
            log.AppendText("! ");
            color = Hud.Ink;
        }
        log.SelectionColor = color;
        log.AppendText(line + "\n");
        log.ScrollToCaret();
    }

    // ---------- updates ----------

    // Shows the banner above the log. Buttons: "update" (What's new, Update now, Later),
    // "news" (What's new, Close), "busy" (none) or "info" (Close).
    void ShowBanner(Func<string> text, string buttons)
    {
        bannerText = text;
        bannerWhatsNew.Visible = buttons == "update" || buttons == "news";
        bannerUpdate.Visible = buttons == "update";
        bannerLater.Visible = buttons != "busy";
        bannerLater.Text = buttons == "update" ? "Later" : "Close";
        if (!bannerShown && WindowState == FormWindowState.Normal) Height += (int)(56 * hud.Zoom);
        bannerShown = true;
        Arrange();
    }

    void SetBannerText(Func<string> text)
    {
        bannerText = text;
        hud.Redraw();
    }

    void HideBanner()
    {
        if (bannerShown && WindowState == FormWindowState.Normal) Height = Math.Max(MinimumSize.Height, Height - (int)(56 * hud.Zoom));
        bannerShown = false;
        foreach (var b in new[] { bannerWhatsNew, bannerUpdate, bannerLater }) b.Visible = false;
        Arrange();
    }

    static WebClient NewWebClient()
    {
        var web = new WebClient { Encoding = Encoding.UTF8 };
        web.Headers[HttpRequestHeader.UserAgent] = "SpoolUp/" + CurrentVersion; // GitHub requires one
        return web;
    }

    // "1.2.0" or "v1.2.0-beta" -> 1.2.0; null if it isn't a version.
    static Version ParseVersion(string text)
    {
        string v = (text ?? "").Trim().TrimStart('v', 'V');
        int cut = v.IndexOfAny(new[] { '-', '+', ' ' });
        if (cut >= 0) v = v.Substring(0, cut);
        Version result;
        return Version.TryParse(v, out result) ? result : null;
    }

    // Asks GitHub for the latest release in the background. userAsked: report "up to date"
    // and errors too, and show a version the player dismissed with Later.
    void CheckForUpdates(bool userAsked)
    {
        if (checking || installer != null) return;
        checking = true;
        ThreadPool.QueueUserWorkItem(delegate
        {
            Dictionary<string, object> release = null;
            string error = null;
            try
            {
                using (var web = NewWebClient())
                {
                    web.Headers[HttpRequestHeader.Accept] = "application/vnd.github+json";
                    release = new JavaScriptSerializer().DeserializeObject(web.DownloadString(LatestReleaseApi)) as Dictionary<string, object>;
                }
            }
            catch (Exception ex) { error = ex.Message; }
            OnUi(delegate { checking = false; OnLatestRelease(release, error, userAsked); });
        });
    }

    void OnLatestRelease(Dictionary<string, object> release, string error, bool userAsked)
    {
        if (release == null)
        {
            if (userAsked) Notify(Lang.T("Couldn't check for updates"), error ?? Lang.T("GitHub didn't answer."));
            return;
        }
        string latest = Str(release, "tag_name").TrimStart('v', 'V');
        Version remote = ParseVersion(latest), local = ParseVersion(CurrentVersion);
        if (remote == null || local == null || remote <= local)
        {
            if (userAsked) Notify(Lang.T("SpoolUp is up to date"), Lang.F("You have version {0}, the latest.", CurrentVersion));
            return;
        }
        // The installer and its checksum, attached to the release by .github/workflows/release.yml.
        string setupName = "SpoolUp-Setup-" + latest + ".exe";
        string setupUrl = "", shaUrl = "";
        foreach (var a in List(release, "assets"))
        {
            var asset = a as Dictionary<string, object>;
            if (asset == null) continue;
            string url = Str(asset, "browser_download_url");
            if (!url.StartsWith(DownloadPrefix, StringComparison.Ordinal)) continue;
            if (Str(asset, "name") == setupName) setupUrl = url;
            else if (Str(asset, "name") == setupName + ".sha256") shaUrl = url;
        }
        if (setupUrl == "" || shaUrl == "")
        {
            if (userAsked) Notify(Lang.T("Update not ready yet"), Lang.F("Version {0} is out, but its installer isn't attached yet. Try again in a few minutes.", latest));
            return;
        }
        if (!userAsked && latest == dismissedVersion) return;
        updateVersion = latest;
        updateSetupUrl = setupUrl;
        updateShaUrl = shaUrl;
        whatsNewUrl = Str(release, "html_url") != "" ? Str(release, "html_url") : ReleasesPage;
        ShowBanner(() => Lang.F("SpoolUp {0} is available (you have {1}).", latest, CurrentVersion), "update");
        if (!Visible) Notify(Lang.F("SpoolUp {0} is available", latest), Lang.T("Open SpoolUp and click Update now."));
    }

    void Notify(string title, string text)
    {
        tray.ShowBalloonTip(5000, title, text, ToolTipIcon.Info);
    }

    void ShowTrayText(Func<string> text)
    {
        trayText = text;
        tray.Text = text();
    }

    void BuildTrayMenu()
    {
        var old = tray.ContextMenuStrip;
        var menu = new ContextMenuStrip();
        menu.Items.Add(Lang.T("Show SpoolUp"), null, delegate { ShowFromTray(); });
        menu.Items.Add(Lang.T("Copy link"), null, delegate { CopyLink(); });
        menu.Items.Add(Lang.T("Edit layout"), null, delegate { OpenEditor(); });
        menu.Items.Add(Lang.T("Check for updates now"), null, delegate { CheckForUpdates(true); });
        menu.Items.Add(Lang.T("Feedback & ideas"), null, delegate { Process.Start(FeedbackPage); });
        menu.Items.Add(new ToolStripSeparator());
        menu.Items.Add(Lang.T("Quit"), null, delegate { Close(); });
        tray.ContextMenuStrip = menu;
        if (old != null) old.Dispose();
    }

    // After the language changes in Settings. The panels translate as they're drawn; the
    // tray menu and the voice commands window are built with their text, so build them again.
    void ApplyLanguage()
    {
        BuildTrayMenu();
        tray.Text = trayText();
        if (commandsWindow != null) commandsWindow.Close();
        hud.Redraw();
    }

    // Downloads the installer for updateVersion, then VerifyAndInstall.
    void InstallUpdate()
    {
        if (updateSetupUrl == "") return;
        string downloading = updateVersion;
        ShowBanner(() => Lang.F("Downloading SpoolUp {0}...", downloading), "busy");
        string dir = UpdateFolder;
        string file = Path.Combine(dir, "SpoolUp-Setup-" + updateVersion + ".exe");
        try { Directory.CreateDirectory(dir); }
        catch (Exception ex) { UpdateFailed(() => ex.Message); return; }
        var web = NewWebClient();
        web.DownloadProgressChanged += (s, e) =>
        {
            int percent = e.ProgressPercentage;
            SetBannerText(() => Lang.F("Downloading SpoolUp {0}...", downloading) + " " + percent + "%");
        };
        web.DownloadFileCompleted += (s, e) =>
        {
            web.Dispose();
            if (e.Error != null) { string error = e.Error.Message; UpdateFailed(() => Lang.F("the download failed ({0})", error)); }
            else VerifyAndInstall(file);
        };
        web.DownloadFileAsync(new Uri(updateSetupUrl), file);
    }

    // Checks the download against the release's SHA-256 file, then runs the installer
    // quietly. It asks this window to quit before it replaces the files, and starts
    // SpoolUp again afterwards. Until then the deck keeps working.
    void VerifyAndInstall(string file)
    {
        SetBannerText(() => Lang.T("Checking the download..."));
        try
        {
            string expected;
            using (var web = NewWebClient())
                expected = web.DownloadString(updateShaUrl).Trim().Split(' ')[0].ToLowerInvariant();
            string actual;
            using (var sha = SHA256.Create())
            using (var stream = File.OpenRead(file))
                actual = BitConverter.ToString(sha.ComputeHash(stream)).Replace("-", "").ToLowerInvariant();
            if (expected.Length != 64 || actual != expected)
            {
                File.Delete(file);
                UpdateFailed(() => Lang.T("the download didn't match its checksum, so it wasn't installed"));
                return;
            }
        }
        catch (Exception ex)
        {
            UpdateFailed(() => Lang.F("couldn't check the download ({0})", ex.Message));
            return;
        }
        string installing = updateVersion;
        SetBannerText(() => Lang.F("Installing SpoolUp {0}. Allow the Windows prompt to continue.", installing));
        try
        {
            installer = Process.Start(new ProcessStartInfo(file, "/SILENT /SUPPRESSMSGBOXES /NORESTART /RELAUNCH=1") { UseShellExecute = true });
        }
        catch (Win32Exception ex)
        {
            UpdateFailed(() => ex.NativeErrorCode == 1223 ? Lang.T("the Windows permission prompt was canceled") : ex.Message);
            return;
        }
        // Normally the installer closes this window. If it ends first, the update didn't happen.
        installer.EnableRaisingEvents = true;
        installer.Exited += delegate
        {
            OnUi(delegate
            {
                int code = installer.ExitCode;
                installer = null;
                UpdateFailed(() => code == 0 ? Lang.T("the installer finished without updating") : Lang.F("the installer stopped (code {0}). Was the Windows prompt canceled?", code));
            });
        };
    }

    void UpdateFailed(Func<string> why)
    {
        Func<string> text = () =>
        {
            string reason = why();
            return Lang.F("Update failed: {0}", reason) + (reason.EndsWith("?") || reason.EndsWith(".") ? "" : ".");
        };
        ShowBanner(text, "info");
        AppendLog(text(), Hud.Warn);
    }

    // Where Update now saves the installer; emptied when the window next starts. It keeps
    // the old name: an older version updating to this one downloads into it too.
    static readonly string UpdateFolder = Path.Combine(Path.GetTempPath(), "TabletDeck-Update");

    // After an update, offers the new version's notes once and removes the used installer.
    void ShowWhatsNewAfterUpdate()
    {
        try { if (Directory.Exists(UpdateFolder)) Directory.Delete(UpdateFolder, true); }
        catch (Exception) { }
        try
        {
            using (var key = Registry.CurrentUser.CreateSubKey(AppKey))
            {
                string last = key.GetValue("LastVersion") as string;
                if (last != null && last != CurrentVersion)
                {
                    whatsNewUrl = ReleasesPage + "/tag/v" + CurrentVersion;
                    ShowBanner(() => Lang.F("Updated to SpoolUp {0}.", CurrentVersion), "news");
                }
                key.SetValue("LastVersion", CurrentVersion);
            }
        }
        catch (Exception) { }
    }

    void OpenEditor()
    {
        if (editorUrl == "" || !serverReady)
        {
            Notify(Lang.T("SpoolUp isn't running"), Lang.T("Start it first; the layout editor is part of the server."));
            return;
        }
        Process.Start(editorUrl);
    }

    // ---------- settings ----------

    string ConfigFile()
    {
        return settingsPath != "" ? settingsPath
            : Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.ApplicationData), "TabletDeck", "config.json");
    }

    // The settings dialog. It starts from config.json; changes go to the running server
    // ("set name value" on stdin), or, while it's stopped, through TabletDeckServer.exe --set.
    void OpenSettings()
    {
        Dictionary<string, object> current = null;
        try { current = new JavaScriptSerializer().DeserializeObject(File.ReadAllText(ConfigFile())) as Dictionary<string, object>; }
        catch (Exception) { }
        bool running = serverReady && server != null && !server.HasExited;
        using (var dialog = new SettingsWindow(current ?? new Dictionary<string, object>(), serverArgs.Contains("--port"), Icon,
            Path.Combine(appDir, "keybinds", "TabletDeck-StarCitizen.xml"), publicNetwork, running ? readyGame : null))
        {
            var result = dialog.ShowDialog(this);
            if (result == DialogResult.OK && dialog.LanguageChanged) ApplyLanguage();
            if (result != DialogResult.OK || dialog.Changes.Count == 0) return;
            if (serverReady && server != null && !server.HasExited)
            {
                foreach (var change in dialog.Changes) SendToServer("set " + change.Key + " " + change.Value);
                if (dialog.NeedsRestart)
                {
                    AppendLog(Lang.T("Restarting the server for the new settings..."), Hud.Dim);
                    StopServer();
                    StartServer();
                }
                return;
            }
            var pairs = new List<string>();
            // Quoted, since some values have spaces (e.g. "pttKeyName=Numpad 5"); none have quotes.
            foreach (var change in dialog.Changes) pairs.Add(GameFolder.Quote(change.Key + "=" + change.Value));
            try
            {
                var psi = new ProcessStartInfo(Path.Combine(appDir, "TabletDeckServer.exe"), "--set " + string.Join(" ", pairs.ToArray()))
                {
                    UseShellExecute = false,
                    CreateNoWindow = true,
                    RedirectStandardError = true,
                    StandardErrorEncoding = Encoding.UTF8,
                };
                using (var proc = Process.Start(psi))
                {
                    string errors = proc.StandardError.ReadToEnd();
                    proc.WaitForExit(10000);
                    if (errors.Trim() != "") AppendLog(errors.Trim(), Hud.Warn);
                }
            }
            catch (Exception ex)
            {
                AppendLog(Lang.F("Couldn't save the settings: {0}", ex.Message), Hud.Warn);
                return;
            }
            if (server == null) StartServer();
        }
    }

    // ---------- buttons ----------

    void CopyLink()
    {
        if (link == "") return;
        try
        {
            Clipboard.SetText(link);
            copyButton.Text = "Copied!";
            hud.Redraw();
            copiedTimer.Stop();
            copiedTimer.Start();
        }
        catch (ExternalException) { }
    }

    void OpenSettingsFolder()
    {
        string dir = settingsPath != "" ? Path.GetDirectoryName(settingsPath)
            : Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.ApplicationData), "TabletDeck");
        Directory.CreateDirectory(dir);
        Process.Start("explorer.exe", "\"" + dir + "\"");
    }

    static bool RunsAtStartup()
    {
        using (var key = Registry.CurrentUser.OpenSubKey(RunKey))
            return key != null && key.GetValue(RunValue) != null;
    }

    static void SetRunAtStartup(bool on)
    {
        using (var key = Registry.CurrentUser.CreateSubKey(RunKey))
        {
            if (on) key.SetValue(RunValue, "\"" + Application.ExecutablePath + "\" --minimized");
            else key.DeleteValue(RunValue, false);
        }
    }

    // ---------- tray and window ----------

    void ShowFromTray()
    {
        Show();
        if (WindowState == FormWindowState.Minimized) WindowState = FormWindowState.Normal;
        Activate();
    }

    // "Start with Windows" opens straight into the tray.
    protected override void SetVisibleCore(bool value)
    {
        if (startHidden)
        {
            startHidden = false;
            value = false;
        }
        base.SetVisibleCore(value);
    }

    protected override void OnResize(EventArgs e)
    {
        base.OnResize(e);
        if (WindowState != FormWindowState.Minimized) return;
        Hide();
        if (!trayHintShown)
        {
            trayHintShown = true;
            tray.ShowBalloonTip(3000, Lang.T("SpoolUp is still running"), Lang.T("Double-click the tray icon to open it, or right-click it to quit."), ToolTipIcon.Info);
        }
    }

    protected override void OnFormClosing(FormClosingEventArgs e)
    {
        // Closing the window quits: the server releases held keys and stops.
        quitting = true;
        StopServer();
        tray.Visible = false;
        base.OnFormClosing(e);
    }

    protected override void WndProc(ref Message m)
    {
        if (m.Msg == Program.ShowWindowMessage) ShowFromTray();
        else if (m.Msg == Program.QuitMessage) BeginInvoke(new MethodInvoker(Close));
        base.WndProc(ref m);
    }
}

// Everything the voice listener understands, with a search box: what to say and which
// button it presses. The list comes from the server, so it matches the current layout.
class VoiceCommandsWindow : HudDialog
{
    [DllImport("user32.dll", CharSet = CharSet.Unicode)]
    static extern IntPtr SendMessage(IntPtr hWnd, int msg, IntPtr wParam, string lParam);

    readonly List<string[]> commands;
    readonly PanelPart panel;
    readonly Note note, presses;
    readonly TextBox search = new TextBox();
    readonly ListView list = new ListView();

    public VoiceCommandsWindow(List<string[]> commands, Icon icon) : base("Voice commands", icon, 540, 580)
    {
        this.commands = commands;
        MinimumSize = SizeFromClientSize(new Size((int)(440 * hud.Zoom), (int)(340 * hud.Zoom)));
        StartPosition = FormStartPosition.CenterScreen;

        panel = Panel("COMMANDS", 16, 16, 508, 548);
        note = Hint("Hold the mic button on the tablet and say one of these. Most Star Citizen controls are toggles, " +
            "so \"engines on\" and \"engines off\" press the same key.", 32, 56, 476, 54);
        Field(search, 32, 116, 476, 32);
        search.TextChanged += delegate { Fill(); };
        search.HandleCreated += delegate { SendMessage(search.Handle, 0x1501, (IntPtr)1, Lang.T("Search, e.g. shields or quantum")); }; // EM_SETCUEBANNER
        Caption("Say", 38, 160);
        presses = Caption("Presses", 300, 160);
        Field(list, 32, 180, 476, 368);
        list.View = View.Details;
        list.FullRowSelect = true;
        list.HeaderStyle = ColumnHeaderStyle.None; // the captions above are the headings
        list.Font = new Font("Segoe UI", 10f);
        list.Columns.Add("Say", 262);
        list.Columns.Add("Presses", 200);
        ResumeLayout(false);
        Cancel = Close;
    }

    protected override void Arrange()
    {
        var area = hud.Area;
        float w = area.Width - 32, h = area.Height - 32;
        panel.Box = new RectangleF(16, 16, w, h);
        note.Box = new RectangleF(32, 56, w - 32, 54);
        Frame(search, new RectangleF(32, 116, w - 32, 32));
        presses.Box.X = 38 + (w - 44) * 0.55f;
        Frame(list, new RectangleF(32, 180, w - 32, h - 180));
        // Column widths aren't scaled with the window, so set them for its size and the screen.
        float inner = (w - 32 - 26) * hud.Zoom;
        if (list.Columns.Count == 2)
        {
            list.Columns[0].Width = (int)(inner * 0.55f);
            list.Columns[1].Width = (int)(inner * 0.45f);
        }
    }

    protected override void OnShown(EventArgs e)
    {
        base.OnShown(e);
        search.Focus();
    }

    // Shows the commands matching the search box, by phrase or button name.
    public void Fill()
    {
        string q = search.Text.Trim().ToLowerInvariant();
        list.BeginUpdate();
        list.Items.Clear();
        foreach (var c in commands)
            if (q == "" || c[0].Contains(q) || c[1].ToLowerInvariant().Contains(q))
                list.Items.Add(new ListViewItem(new[] { c[0], c[1] }));
        list.EndUpdate();
        Text = commands.Count == 0 ? Lang.T("Voice commands (not running)")
            : q == "" ? Lang.F("Voice commands ({0})", commands.Count)
            : Lang.F("Voice commands ({0} of {1})", list.Items.Count, commands.Count);
    }
}

// The Star Citizen folder, as TabletDeckServer.exe --find-game finds it (findInstalls and
// gameInUse in server.js), so Settings, the keybind profile and the server use the same copy.
static class GameFolder
{
    // The game's release channels, as SC_CHANNELS in server.js.
    public static readonly string[] Channels = { "LIVE", "PTU", "EPTU", "TECH-PREVIEW", "HOTFIX" };

    // The channel folders in a folder the player picked: the StarCitizen folder, a channel
    // folder inside it, or the folder above it.
    public static List<string> ChannelsIn(string picked)
    {
        var found = new List<string>();
        if (Array.IndexOf(Channels, Path.GetFileName(picked.TrimEnd('\\')).ToUpperInvariant()) >= 0) found.Add(picked);
        foreach (var root in new[] { picked, Path.Combine(picked, "StarCitizen") })
            foreach (var channel in Channels)
                if (Directory.Exists(Path.Combine(root, channel))) found.Add(Path.Combine(root, channel));
        return found;
    }

    // { folder, game, installs } from --find-game, or null when the server can't answer.
    public static Dictionary<string, object> Find(string serverExe)
    {
        try
        {
            if (!File.Exists(serverExe)) return null;
            var psi = new ProcessStartInfo(serverExe, "--find-game")
            {
                UseShellExecute = false,
                CreateNoWindow = true,
                RedirectStandardOutput = true,
                StandardOutputEncoding = Encoding.UTF8,
            };
            using (var proc = Process.Start(psi))
            {
                string output = proc.StandardOutput.ReadToEnd();
                proc.WaitForExit(15000);
                foreach (var line in output.Split('\n'))
                    if (line.Trim().StartsWith("{"))
                        return new JavaScriptSerializer().DeserializeObject(line.Trim()) as Dictionary<string, object>;
            }
        }
        catch (Exception) { }
        return null;
    }

    public static string Text(Dictionary<string, object> d, string key)
    {
        object v;
        return d != null && d.TryGetValue(key, out v) && v != null ? Convert.ToString(v, CultureInfo.InvariantCulture) : "";
    }

    // "LIVE: C:\...\StarCitizen\LIVE" for a game from --find-game or the server's "ready".
    public static string Describe(Dictionary<string, object> game)
    {
        return Text(game, "channel") + ": " + Text(game, "dir");
    }

    // The channel folders --find-game listed.
    public static List<string> Installs(Dictionary<string, object> found)
    {
        var dirs = new List<string>();
        object v;
        if (found != null && found.TryGetValue("installs", out v) && v is object[])
            foreach (var item in (object[])v)
            {
                string dir = Text(item as Dictionary<string, object>, "dir");
                if (dir != "") dirs.Add(dir);
            }
        return dirs;
    }

    // One command-line argument in quotes, with backslashes before a quote doubled as
    // Windows expects (a folder like D:\ ends in one).
    public static string Quote(string arg)
    {
        var sb = new StringBuilder("\"");
        int slashes = 0;
        foreach (char c in arg)
        {
            if (c == '\\') { slashes++; continue; }
            sb.Append('\\', c == '"' ? slashes * 2 + 1 : slashes);
            sb.Append(c);
            slashes = 0;
        }
        sb.Append('\\', slashes * 2);
        return sb.Append('"').ToString();
    }
}

// Installs the SpoolUp Keyboard profile into Star Citizen's Controls\Mappings folder, for each
// copy of the game (LIVE, PTU and so on) the player ticks. Listed as the server finds them
// (GameFolder, the folder set in Settings or the usual places), or picked by hand. A copy with the player's own actionmaps.xml gets "SpoolUp Keyboard (your
// binds)" instead, which TabletDeckServer.exe --merge-profile builds from it: every bind the
// player has, plus keys only for the deck's actions they haven't bound. A copy without one
// gets the plain profile (keybinds\TabletDeck-StarCitizen.xml in the install folder).
class KeybindProfileWindow : HudDialog
{
    const string ProfileName = "TabletDeck-StarCitizen.xml";
    const string MergedName = "SpoolUp-YourBinds.xml";

    readonly string profile;
    readonly string server; // TabletDeckServer.exe, which builds the merged profile
    readonly List<string> games = new List<string>(); // ...\StarCitizen\<channel>, same order as list
    readonly CheckedListBox list = new CheckedListBox();
    readonly Note status;

    public KeybindProfileWindow(string profile, Icon icon) : base("Install keybind profile", icon, 560, 644)
    {
        this.profile = profile;
        server = Path.Combine(Path.GetDirectoryName(Path.GetDirectoryName(profile)), "TabletDeckServer.exe");
        FormBorderStyle = FormBorderStyle.FixedDialog;
        MaximizeBox = false;
        MinimizeBox = false;

        Panel("STAR CITIZEN COPIES", 16, 16, 528, 320);
        var intro = Hint("Copies the SpoolUp Keyboard profile into Star Citizen, so buttons for actions with no default key work. " +
            "A copy of the game with binds of your own gets \"SpoolUp Keyboard (your binds)\" instead: all of your binds, plus keys only for the deck's actions you haven't bound. " +
            "Tick the copies to install it into.", 32, 56, 496, 98);
        intro.Color = Hud.Ink;
        Field(list, 32, 160, 496, 120);
        list.CheckOnClick = true;
        list.HorizontalScrollbar = true;
        list.Font = new Font("Segoe UI", 10f);
        Button("Choose game folder...", 32, 292, 220, delegate { ChooseFolder(); });

        Panel("STATUS", 16, 348, 528, 236);
        status = Hint("", 32, 388, 496, 188);

        var install = Primary("Install", 336, 596, 100, delegate { Install(); });
        Button("Close", 444, 596, 100, delegate { Close(); });
        Accept = delegate { if (install.Enabled) Install(); };
        Cancel = Close;
        ResumeLayout(false);

        if (!File.Exists(profile))
        {
            ShowStatus(Lang.F("The profile file is missing ({0}). Reinstall SpoolUp.", profile), Hud.Bad);
            install.Enabled = false;
            return;
        }
        foreach (var game in FindGames()) AddGame(game);
        ShowStatus(games.Count == 0
            ? Lang.T("Star Citizen wasn't found. Click \"Choose game folder...\" and pick the StarCitizen folder (the one with LIVE in it).")
            : Lang.F(games.Count == 1 ? "Found {0} copy of the game. Missing one? Click \"Choose game folder...\"."
                : "Found {0} copies of the game. Missing one? Click \"Choose game folder...\".", games.Count), Hud.Dim);
    }

    // The status in Ink, with its state as a dot: Dim for plain news.
    void ShowStatus(string text, Color color)
    {
        status.Text = text;
        status.Mark = color == Hud.Dim ? Color.Empty : color;
        status.Color = Hud.Ink;
        Redraw();
    }

    // ...\StarCitizen\<channel> folders, as the server finds them.
    List<string> FindGames()
    {
        return GameFolder.Installs(GameFolder.Find(server));
    }

    static string Target(string game, string name)
    {
        return Path.Combine(game, "user", "client", "0", "Controls", "Mappings", name);
    }

    // The player's own binds for this copy of the game, which the game writes as they rebind.
    static string Actionmaps(string game)
    {
        return Path.Combine(game, "user", "client", "0", "Profiles", "default", "actionmaps.xml");
    }

    bool CanMerge(string game)
    {
        return File.Exists(Actionmaps(game)) && File.Exists(server);
    }

    // Adds a game copy to the list, ticked unless it already has this exact plain profile.
    void AddGame(string game)
    {
        foreach (var g in games) if (string.Equals(g, game, StringComparison.OrdinalIgnoreCase)) return;
        string state;
        bool tick;
        string target = Target(game, ProfileName);
        if (CanMerge(game)) { state = "your binds found, gets SpoolUp Keyboard (your binds)"; tick = true; }
        else if (!File.Exists(target)) { state = "not installed"; tick = true; }
        else if (SameFile(target, profile)) { state = "already up to date"; tick = false; }
        else { state = "older copy, will be replaced"; tick = true; }
        games.Add(game);
        list.Items.Add(Path.GetFileName(game) + "  -  " + game + "  (" + Lang.T(state) + ")", tick);
    }

    static bool SameFile(string a, string b)
    {
        try { return File.ReadAllText(a).Replace("\r\n", "\n") == File.ReadAllText(b).Replace("\r\n", "\n"); }
        catch (Exception) { return false; }
    }

    // Accepts the StarCitizen folder, a channel folder inside it, or the folder above it.
    void ChooseFolder()
    {
        using (var dialog = new FolderBrowserDialog { Description = Lang.T("Pick the StarCitizen folder (the one with LIVE in it)."), ShowNewFolderButton = false })
        {
            if (dialog.ShowDialog(this) != DialogResult.OK) return;
            var candidates = GameFolder.ChannelsIn(dialog.SelectedPath);
            if (candidates.Count == 0)
            {
                ShowStatus(Lang.T("That folder doesn't look like Star Citizen. Pick the StarCitizen folder, the one with LIVE in it."), Hud.Warn);
                return;
            }
            foreach (var game in candidates) AddGame(game);
            ShowStatus(Lang.T("Added. Tick the copies of the game to install the profile into, then click Install."), Hud.Dim);
        }
    }

    // Runs TabletDeckServer.exe --merge-profile for one copy. Returns its exit code (0 built,
    // 3 no actionmaps.xml after all) and sets added / moved from its first line, "added N moved M",
    // and error to what it printed on failure.
    int Merge(string game, out int added, out int moved, out string error)
    {
        added = 0;
        moved = 0;
        error = "";
        var psi = new ProcessStartInfo(server, "--merge-profile \"" + Target(game, MergedName) + "\" \"" + Actionmaps(game) + "\"")
        {
            UseShellExecute = false,
            CreateNoWindow = true,
            RedirectStandardOutput = true,
            RedirectStandardError = true,
            StandardOutputEncoding = Encoding.UTF8,
            StandardErrorEncoding = Encoding.UTF8,
        };
        using (var proc = Process.Start(psi))
        {
            var errors = new StringBuilder();
            proc.ErrorDataReceived += delegate(object sender, DataReceivedEventArgs e) { if (e.Data != null) lock (errors) errors.AppendLine(e.Data); };
            proc.BeginErrorReadLine();
            string output = proc.StandardOutput.ReadToEnd();
            proc.WaitForExit(); // also waits for the error lines
            lock (errors) error = errors.ToString().Trim();
            var first = output.Split('\n')[0].Trim().Split(' ');
            if (first.Length >= 4 && first[0] == "added")
            {
                int.TryParse(first[1], out added);
                int.TryParse(first[3], out moved);
            }
            return proc.ExitCode;
        }
    }

    void Install()
    {
        var plain = new List<string>();
        var merged = new List<string>();
        var failed = new List<string>();
        int movedTotal = 0;
        for (int i = 0; i < games.Count; i++)
        {
            if (!list.GetItemChecked(i)) continue;
            string name = Path.GetFileName(games[i]);
            try
            {
                if (CanMerge(games[i]))
                {
                    int added, moved;
                    string error;
                    int code = Merge(games[i], out added, out moved, out error);
                    if (code == 0)
                    {
                        merged.Add(Lang.F(added == 1 ? "{0} (1 key added)" : "{0} ({1} keys added)", name, added));
                        movedTotal += moved;
                        continue;
                    }
                    if (code != 3) // 3: no actionmaps.xml after all, so the plain profile below
                    {
                        failed.Add(name + ": " + (error != "" ? error : Lang.F("exit code {0}", code)));
                        continue;
                    }
                }
                string target = Target(games[i], ProfileName);
                Directory.CreateDirectory(Path.GetDirectoryName(target));
                File.Copy(profile, target, true);
                plain.Add(name);
            }
            catch (Exception ex)
            {
                failed.Add(name + ": " + ex.Message);
            }
        }
        if (plain.Count == 0 && merged.Count == 0 && failed.Count == 0)
        {
            ShowStatus(Lang.T("Nothing ticked. Tick the copies of the game to install the profile into."), Hud.Warn);
            return;
        }
        var text = new StringBuilder();
        if (merged.Count > 0)
        {
            text.Append(Lang.F("Made \"SpoolUp Keyboard (your binds)\" for {0}. ", string.Join(", ", merged.ToArray())));
            text.Append(Lang.T("Now, in Star Citizen: Options > Keybindings > Control Profiles, load it. If you change binds in the game before that, install again first. "));
            if (movedTotal > 0)
                text.Append(Lang.T("Your binds use some of the deck's usual keys, so free ones were picked: reload the deck on the tablet after loading the profile. "));
        }
        if (plain.Count > 0)
        {
            text.Append((text.Length > 0 ? "\n\n" : "") + Lang.F("Installed into {0}. ", string.Join(", ", plain.ToArray())));
            text.Append(Lang.T("Now, in Star Citizen: Options > Keybindings > Control Profiles, load \"SpoolUp Keyboard\". "));
            text.Append(Lang.T("This replaces your current custom binds, so note any you want to keep and set them again afterwards."));
        }
        if (failed.Count > 0)
            text.Append((text.Length > 0 ? "\n\n" : "") + Lang.F("Couldn't copy into {0}. Try running SpoolUp as administrator, or copy {1} by hand.",
                string.Join("; ", failed.ToArray()), profile));
        ShowStatus(text.ToString(), failed.Count > 0 ? Hud.Warn : Hud.Good);
        // Refresh the states shown in the list.
        var again = new List<string>(games);
        games.Clear();
        list.Items.Clear();
        foreach (var game in again) AddGame(game);
    }
}

// Settings that would otherwise mean editing config.json. Only what changed ends up in
// Changes, as setting name -> text value (see SETTINGS in server.js).
class SettingsWindow : HudDialog
{
    public readonly List<KeyValuePair<string, string>> Changes = new List<KeyValuePair<string, string>>();
    public bool NeedsRestart; // port or voice changes take a server restart

    readonly Dictionary<string, object> current;
    readonly TextBox code = new TextBox();
    readonly NumericUpDown port = new NumericUpDown();
    readonly HudView.Item focus;
    readonly NumericUpDown tap = new NumericUpDown();
    readonly HudView.Item voiceOn;
    readonly ComboBox language = new ComboBox();
    readonly ComboBox pttMode = new ComboBox();
    readonly TextBox links = new TextBox();
    readonly ComboBox windowLanguage = new ComboBox();
    // The Star Citizen folder: the saved one ("" finds it automatically) and the one Save keeps.
    readonly string savedFolder;
    string gameFolder;
    readonly string serverExe;
    readonly Note gameNote;
    readonly HudView.Item resetFolder;
    public bool LanguageChanged; // saved already; the window shows it once Settings closes

    // config.json's links as editable lines.
    string LinksText()
    {
        object v;
        var lines = new List<string>();
        if (current.TryGetValue("links", out v) && v is object[])
            foreach (var item in (object[])v)
            {
                var l = item as Dictionary<string, object>;
                if (l == null) continue;
                lines.Add(Convert.ToString(l["name"], CultureInfo.InvariantCulture) + " | " + Convert.ToString(l["url"], CultureInfo.InvariantCulture));
            }
        return string.Join("\r\n", lines.ToArray());
    }

    // The lines as the JSON the server takes, or null (with a message) if one isn't valid.
    string LinksJson()
    {
        var list = new List<Dictionary<string, string>>();
        foreach (var raw in links.Lines)
        {
            string line = raw.Trim();
            if (line == "") continue;
            int bar = line.IndexOf('|');
            string name = bar > 0 ? line.Substring(0, bar).Trim() : "";
            string url = bar > 0 ? line.Substring(bar + 1).Trim() : "";
            if (name == "" || name.Length > 24 || name.IndexOfAny(new[] { '<', '>', '"' }) >= 0
                || !System.Text.RegularExpressions.Regex.IsMatch(url, "^https://[^\\s\"<>]{3,300}$"))
            {
                MessageBox.Show(this, Lang.F("This link isn't right:\n\n{0}\n\nWrite each one as: Name | https://address (names up to 24 characters).", line), "SpoolUp", MessageBoxButtons.OK, MessageBoxIcon.Warning);
                return null;
            }
            list.Add(new Dictionary<string, string> { { "name", name }, { "url", url } });
        }
        if (list.Count > 12) { MessageBox.Show(this, Lang.T("Up to 12 links, please."), "SpoolUp", MessageBoxButtons.OK, MessageBoxIcon.Warning); return null; }
        return new JavaScriptSerializer().Serialize(list);
    }
    readonly HudView.Item pttSound, shortEmotes, pttButton, discoveryOn;
    int pttKey;          // Windows virtual-key code, 0 = none
    string pttName = "";
    bool capturing;      // waiting for the player to press the push-to-talk key

    // ---------- push-to-talk key ----------

    void StartCapture()
    {
        if (capturing) { capturing = false; ShowPttKey(); return; } // a second click cancels
        capturing = true;
        pttButton.Text = "Press a key or button...";
        Redraw();
    }

    // While capturing, every key (Tab, Enter and Alt included) is the push-to-talk key; Esc cancels.
    protected override bool ProcessCmdKey(ref Message msg, Keys keyData)
    {
        if (!capturing) return base.ProcessCmdKey(ref msg, keyData);
        var key = keyData & Keys.KeyCode;
        if (key == Keys.Escape) { capturing = false; ShowPttKey(); }
        else SetPttKey((int)key);
        return true;
    }

    void SetPttKey(int vk)
    {
        capturing = false;
        pttKey = vk;
        pttName = vk == 0 ? "" : KeyName(vk);
        ShowPttKey();
    }

    void ShowPttKey()
    {
        pttButton.Text = pttKey == 0 ? "None: click to set" : pttName;
        Redraw();
    }

    static string KeyName(int vk)
    {
        if (vk == 4) return "Middle mouse";
        if (vk == 5) return "Mouse 4";
        if (vk == 6) return "Mouse 5";
        string name = ((Keys)vk).ToString();
        if (name.StartsWith("NumPad")) return "Numpad " + name.Substring(6);
        if (name.StartsWith("D") && name.Length == 2 && char.IsDigit(name[1])) return name.Substring(1);
        return System.Text.RegularExpressions.Regex.Replace(name, "[^A-Za-z0-9 +-]", "");
    }

    // Two columns of panels: pairing, keys and voice on the left, push-to-talk, links and
    // network on the right, and the buttons along the bottom. 16 pixels inside each panel,
    // 12 between them.
    public SettingsWindow(Dictionary<string, object> current, bool portFromCommandLine, Icon icon, string keybindProfile, string publicNetwork,
        Dictionary<string, object> runningGame)
        : base("SpoolUp settings", icon, 840, 824)
    {
        this.current = current;
        FormBorderStyle = FormBorderStyle.FixedDialog;
        MaximizeBox = false;
        MinimizeBox = false;

        Panel("PAIRING", 16, 16, 396, 224);
        Caption("Pairing code", 32, 56);
        Field(code, 32, 74, 200, 32);
        code.Font = new Font("Consolas", 12f, FontStyle.Bold);
        code.MaxLength = 32;
        code.Text = Get("token", "");
        Button("New random code", 244, 74, 152, delegate { code.Text = RandomCode(); });
        Hint("Letters and numbers. Tablets already paired keep working until they reload; new ones need the new code.", 32, 114, 364, 38);
        Caption("Port", 32, 162);
        Field(port, 32, 180, 96, 32);
        port.Minimum = 1024;
        port.Maximum = 65535;
        port.Value = Math.Max(1024, Math.Min(65535, GetNumber("port", 8765)));
        port.Enabled = !portFromCommandLine;
        Hint(portFromCommandLine ? "Set with --port when SpoolUp was started, so it can't be changed here."
            : "Changing it restarts the server. Tablets then need the new address or QR code.", 140, 176, 256, 54);

        Panel("KEYS", 16, 252, 396, 154);
        focus = Switch("Only press keys while Star Citizen is in front", GetBool("onlyWhenGameFocused", true), 32, 292, 364);
        focus.Tip = "Off: keys go to whichever window is in front.";
        Caption("Tap length", 32, 328);
        Field(tap, 32, 346, 88, 32);
        tap.Minimum = 20;
        tap.Maximum = 200;
        tap.Value = Math.Max(20, Math.Min(200, GetNumber("tapMs", 60)));
        Hint("milliseconds a tap holds the key. Raise it if the game misses taps.", 132, 342, 264, 54);

        Panel("VOICE", 16, 418, 396, 204);
        voiceOn = Switch("Voice commands", GetBool("voice", true), 32, 458, 364);
        voiceOn.Click = delegate { language.Enabled = shortEmotes.Enabled = voiceOn.On; Redraw(); };
        Caption("Voice language", 32, 494);
        Field(language, 32, 512, 112, 32);
        language.DropDownStyle = ComboBoxStyle.DropDown;
        language.FlatStyle = FlatStyle.Flat;
        language.Items.AddRange(new object[] { "en-US", "en-GB", "en-AU", "en-CA", "en-IN" });
        language.Text = Get("voiceLanguage", "en-US");
        Hint("Needs that English speech recognizer installed in Windows (Settings > Time & Language > Speech).", 156, 508, 240, 72);
        shortEmotes = Switch("Emotes also answer to one word, like \"wave\"", GetBool("shortEmotes", false), 32, 584, 364);
        shortEmotes.Tip = "Without this, emotes need \"emote\" first (\"emote wave\"), so a stray word doesn't set one off.";
        language.Enabled = shortEmotes.Enabled = voiceOn.On;

        // Push-to-talk from a key or mouse button on the PC (see native/voice.cs).
        Panel("PUSH-TO-TALK", 428, 16, 396, 260);
        Caption("Key on this PC", 444, 56);
        pttKey = GetNumber("pttKey", 0);
        pttName = Get("pttKeyName", "");
        pttButton = Button("", 444, 74, 256, delegate { StartCapture(); });
        pttButton.OtherButton = (b) =>
        {
            if (!capturing) return;
            int vk = b == MouseButtons.Middle ? 4 : b == MouseButtons.XButton1 ? 5 : b == MouseButtons.XButton2 ? 6 : 0;
            if (vk != 0) SetPttKey(vk);
        };
        Button("None", 712, 74, 96, delegate { SetPttKey(0); });
        Caption("Mode", 444, 118);
        Field(pttMode, 444, 136, 364, 32);
        pttMode.DropDownStyle = ComboBoxStyle.DropDownList;
        pttMode.FlatStyle = FlatStyle.Flat;
        pttMode.Items.AddRange(new object[] { Lang.T("Hold to talk"), Lang.T("Press to start, again to stop") });
        pttMode.SelectedIndex = Get("pttMode", "hold") == "toggle" ? 1 : 0;
        pttSound = Switch("Play a sound when it starts and stops listening", GetBool("pttSound", true), 444, 178, 364);
        Hint("Pick a key Star Citizen doesn't use, like a mouse side button, a numpad key or F13-F24: the key still reaches the game.", 444, 210, 364, 56);
        ShowPttKey();

        // Websites the tablet's "Open on PC" buttons open here, one per line: "Name | https://...".
        Panel("LINKS", 428, 288, 396, 178);
        Caption("Websites the tablet can open on this PC", 444, 328);
        Field(links, 444, 346, 364, 64);
        links.Multiline = true;
        links.ScrollBars = ScrollBars.Vertical;
        links.Font = new Font("Consolas", 10f);
        links.Text = LinksText();
        Hint("One per line: Name | https://address. The tablet shows the names, and this PC opens the addresses.", 444, 418, 364, 40);

        // Why tablets need a Private network, or, while Windows calls it Public, that it's
        // blocking them now. The button is the main window's banner button.
        Panel("NETWORK", 428, 478, 396, 182);
        var network = Hint(publicNetwork != ""
            ? Lang.F("Windows treats \"{0}\" as a Public network. Set it to Private so tablets on your network can reach SpoolUp.", publicNetwork)
            : Lang.T("Tablets connect over your home network. Windows has to treat it as a Private network: on a Public one it blocks them."),
            444, 518, 364, 56);
        if (publicNetwork != "") network.Mark = Hud.Warn;
        var networkButton = Button("Open network settings", 444, 580, 232, delegate { DeckWindow.OpenNetworkSettings(publicNetwork); });
        networkButton.Tip = "Pick your network there and set its network profile to Private";
        // The SpoolUp apps list this PC by itself (mDNS, see discovery.js); off, they need its address.
        discoveryOn = Switch("Let the SpoolUp app find this PC", GetBool("discovery", true), 444, 624, 364);
        discoveryOn.Tip = "Answers the SpoolUp app with this PC's name and address, so it shows up in the app's list.";

        // The Star Citizen folder: found automatically like the server does, or picked here.
        // Keybinds, the keybind profile and Game.log all come from it.
        Panel("STAR CITIZEN", 16, 672, 808, 96);
        gameNote = Hint("", 32, 712, 420, 44);
        Button("Change...", 464, 712, 128, delegate { ChangeFolder(); }).Tip = "Keybinds, the keybind profile and battle alerts use this copy of the game.";
        resetFolder = Button("Reset to automatic", 604, 712, 204, delegate { ResetFolder(); });
        serverExe = Path.Combine(Path.GetDirectoryName(Path.GetDirectoryName(keybindProfile)), "TabletDeckServer.exe");
        object folders;
        savedFolder = current.TryGetValue("gameFolders", out folders) && folders is object[] && ((object[])folders).Length > 0
            ? Convert.ToString(((object[])folders)[0], CultureInfo.InvariantCulture).Trim() : "";
        gameFolder = savedFolder;
        resetFolder.Enabled = gameFolder != "";
        if (runningGame != null) ShowGame(runningGame);
        else
        {
            gameNote.Text = Lang.T("Looking for Star Citizen...");
            Load += delegate { LookForGame(); }; // once the window has a handle to report back to
        }

        Button("Install keybind profile...", 16, 776, 236, delegate
        {
            using (var dialog = new KeybindProfileWindow(keybindProfile, Icon)) dialog.ShowDialog(this);
        });
        // The window's own language; the tablet picks its language in Deck settings.
        Caption("Language", 268, 785);
        Field(windowLanguage, 352, 776, 176, 32);
        windowLanguage.DropDownStyle = ComboBoxStyle.DropDownList;
        windowLanguage.FlatStyle = FlatStyle.Flat;
        foreach (var choice in Lang.Choices) windowLanguage.Items.Add(choice[0] == "" ? Lang.T(choice[1]) : choice[1]);
        string saved = Lang.Saved;
        for (int i = 0; i < Lang.Choices.Length; i++) if (Lang.Choices[i][0] == saved) windowLanguage.SelectedIndex = i;
        if (windowLanguage.SelectedIndex < 0) windowLanguage.SelectedIndex = 0;

        Primary("Save", 624, 776, 96, delegate { Save(); });
        Button("Cancel", 728, 776, 96, delegate { DialogResult = DialogResult.Cancel; });
        Accept = Save;
        Cancel = delegate { DialogResult = DialogResult.Cancel; };
        ResumeLayout(false);
    }

    // ---------- Star Citizen folder ----------

    // Asks the server which copy it finds, off the window's thread (it starts a process).
    void LookForGame()
    {
        gameNote.Text = Lang.T("Looking for Star Citizen...");
        var thread = new Thread(() =>
        {
            var found = GameFolder.Find(serverExe);
            try
            {
                BeginInvoke((MethodInvoker)delegate
                {
                    if (gameFolder != savedFolder) return; // the player picked another meanwhile
                    object game;
                    ShowGame(found != null && found.TryGetValue("game", out game) ? game as Dictionary<string, object> : null);
                });
            }
            catch (InvalidOperationException) { } // the window closed first
        });
        thread.IsBackground = true;
        thread.Start();
    }

    // The copy in use, or how to pick one when there's none.
    void ShowGame(Dictionary<string, object> game)
    {
        gameNote.Mark = game == null ? Hud.Warn : Color.Empty;
        gameNote.Color = game == null ? Hud.Ink : Hud.Dim;
        gameNote.Text = game != null ? Lang.F(gameFolder != "" ? "{0}, in the folder you picked." : "{0}, found automatically.", GameFolder.Describe(game))
            : gameFolder != "" ? Lang.F("Nothing found in {0}. Click Change... to pick another folder.", gameFolder)
            : Lang.T("Star Citizen wasn't found. Click Change... and pick the StarCitizen folder (the one with LIVE in it).");
        Redraw();
    }

    void ChangeFolder()
    {
        using (var dialog = new FolderBrowserDialog { Description = Lang.T("Pick the StarCitizen folder (the one with LIVE in it)."), ShowNewFolderButton = false })
        {
            if (gameFolder != "" && Directory.Exists(gameFolder)) dialog.SelectedPath = gameFolder;
            if (dialog.ShowDialog(this) != DialogResult.OK) return;
            if (GameFolder.ChannelsIn(dialog.SelectedPath).Count == 0)
            {
                MessageBox.Show(this, Lang.T("That folder doesn't look like Star Citizen. Pick the StarCitizen folder, the one with LIVE in it."), "SpoolUp", MessageBoxButtons.OK, MessageBoxIcon.Warning);
                return;
            }
            gameFolder = dialog.SelectedPath;
            resetFolder.Enabled = true;
            gameNote.Mark = Color.Empty;
            gameNote.Color = Hud.Dim;
            gameNote.Text = Lang.F("Picked {0}. Click Save to use it.", gameFolder);
            Redraw();
        }
    }

    void ResetFolder()
    {
        gameFolder = "";
        resetFolder.Enabled = false;
        gameNote.Mark = Color.Empty;
        gameNote.Color = Hud.Dim;
        gameNote.Text = Lang.T(savedFolder != "" ? "Back to automatic. Click Save to use it." : "Looking for Star Citizen...");
        if (savedFolder == "") LookForGame();
        Redraw();
    }

    string Get(string key, string fallback)
    {
        object v;
        return current.TryGetValue(key, out v) && v != null ? Convert.ToString(v, CultureInfo.InvariantCulture) : fallback;
    }

    int GetNumber(string key, int fallback)
    {
        int n;
        return int.TryParse(Get(key, ""), out n) ? n : fallback;
    }

    bool GetBool(string key, bool fallback)
    {
        object v;
        return current.TryGetValue(key, out v) && v is bool ? (bool)v : fallback;
    }

    static string RandomCode()
    {
        var bytes = new byte[4];
        using (var rng = new RNGCryptoServiceProvider()) rng.GetBytes(bytes);
        return BitConverter.ToString(bytes).Replace("-", "").ToLowerInvariant();
    }

    // Checks the fields, records what changed, and closes.
    void Save()
    {
        string newCode = code.Text.Trim().ToLowerInvariant();
        if (!System.Text.RegularExpressions.Regex.IsMatch(newCode, "^[a-z0-9]{1,32}$"))
        {
            MessageBox.Show(this, Lang.T("The pairing code can only have letters and numbers (up to 32)."), "SpoolUp", MessageBoxButtons.OK, MessageBoxIcon.Warning);
            code.Focus();
            return;
        }
        string newLanguage = language.Text.Trim();
        if (!System.Text.RegularExpressions.Regex.IsMatch(newLanguage, "^[a-z]{2}(-[A-Za-z]{2,4})?$"))
        {
            MessageBox.Show(this, Lang.T("The voice language should look like en-US."), "SpoolUp", MessageBoxButtons.OK, MessageBoxIcon.Warning);
            language.Focus();
            return;
        }
        Change("token", Get("token", "").Trim().ToLowerInvariant(), newCode, false);
        if (port.Enabled) Change("port", GetNumber("port", 8765).ToString(), ((int)port.Value).ToString(), true);
        Change("onlyWhenGameFocused", OnOff(GetBool("onlyWhenGameFocused", true)), OnOff(focus.On), false);
        Change("tapMs", GetNumber("tapMs", 60).ToString(), ((int)tap.Value).ToString(), false);
        Change("voice", OnOff(GetBool("voice", true)), OnOff(voiceOn.On), true);
        Change("voiceLanguage", Get("voiceLanguage", "en-US"), newLanguage, true);
        if (GetNumber("pttKey", 0) != pttKey)
        {
            Change("pttKey", GetNumber("pttKey", 0).ToString(), pttKey.ToString(), false);
            if (pttKey != 0) Change("pttKeyName", "", pttName == "" ? "Key" : pttName, false);
        }
        Change("pttMode", Get("pttMode", "hold"), pttMode.SelectedIndex == 1 ? "toggle" : "hold", false);
        Change("pttSound", OnOff(GetBool("pttSound", true)), OnOff(pttSound.On), false);
        Change("shortEmotes", OnOff(GetBool("shortEmotes", false)), OnOff(shortEmotes.On), false);
        Change("discovery", OnOff(GetBool("discovery", true)), OnOff(discoveryOn.On), false);
        if (gameFolder != savedFolder) Change("gameFolders", savedFolder, gameFolder == "" ? "auto" : gameFolder, false);
        if (links.Text.Trim() != LinksText().Trim())
        {
            string json = LinksJson();
            if (json == null) { Changes.Clear(); NeedsRestart = false; links.Focus(); return; }
            Change("links", "", json, false);
        }
        string chosen = Lang.Choices[Math.Max(0, windowLanguage.SelectedIndex)][0];
        if (chosen != Lang.Saved)
        {
            Lang.Save(chosen);
            LanguageChanged = true;
        }
        DialogResult = DialogResult.OK;
    }

    void Change(string name, string before, string after, bool restart)
    {
        if (before == after) return;
        Changes.Add(new KeyValuePair<string, string>(name, after));
        if (restart) NeedsRestart = true;
    }

    static string OnOff(bool on)
    {
        return on ? "on" : "off";
    }
}
