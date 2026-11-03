import QtQuick
import Quickshell
import Quickshell.Io
import qs.Commons
import qs.Ui

// NOTE: State container for the Peace panel: playback/settings state, all
// NOTE: `peace` CLI processes, and the actions the view components trigger.
// NOTE: The panel owns wiring; this object owns data and side effects.
// NOTE: Item (not QtObject) so Process/Timer children are legal as data.
Item {
  id: peace

  width: 0
  height: 0

  property var hostWidget: null
  property var bar: null
  property string moduleName: "io.github.sohanemon.omapeace"
  property string peaceExecutable: ""
  property bool active: false

  property string statusText: "Checking playback…"
  property bool artworkCaptured: false
  property bool artworkIsSessionBase: false
  property string artworkCandidate: ""
  property var pendingListenArgs: null
  property string referenceText: "55"
  property bool basmalaEnabled: false
  property bool followEnabled: false
  property bool randomBackground: false
  property bool showTranslation: true
  property bool showWordTranslation: false
  property bool showReference: true
  property bool useThemeColors: false
  property var settings: hostWidget && hostWidget.settings ? hostWidget.settings : null

  property string selectedTranslation: {
    if (settings && settings.translation) return String(settings.translation)
    return "20"
  }
  property string selectedWBWLanguage: {
    if (settings && settings.wbwLanguage) return String(settings.wbwLanguage)
    return "en"
  }

  property bool translationPickerOpen: false
  property var allTranslations: []
  property bool translationsLoaded: false
  property bool translationsLoading: false
  property string translationsError: ""
  property string translationSearchQuery: ""

  property bool wbwPickerOpen: false
  property var allWBWLanguages: []
  property bool wbwLanguagesLoaded: false
  property bool wbwLanguagesLoading: false
  property string wbwLanguagesError: ""
  property string wbwSearchQuery: ""

  readonly property string userLanguage: {
    var langStr = String(Quickshell.env("LC_ALL") || Quickshell.env("LANG") || "").toLowerCase()
    var base = langStr.split(".")[0].split("_")[0]
    return base || "en"
  }

  readonly property var filteredTranslations: {
    var q = peace.translationSearchQuery.toLowerCase()
    var items = peace.allTranslations || []
    var matched = []
    for (var i = 0; i < items.length; i++) {
      var item = items[i]
      if (!q) {
        matched.push(item)
        continue
      }
      var name = String(item.name || "").toLowerCase()
      var author = String(item.author || "").toLowerCase()
      var lang = String(item.language || "").toLowerCase()
      if (name.indexOf(q) !== -1 || author.indexOf(q) !== -1 || lang.indexOf(q) !== -1) {
        matched.push(item)
      }
    }

    var userLang = peace.userLanguage
    var userLangName = ""
    if (userLang === "en") userLangName = "english"
    else if (userLang === "ur") userLangName = "urdu"
    else if (userLang === "bn") userLangName = "bengali"
    else if (userLang === "tr") userLangName = "turkish"
    else if (userLang === "id") userLangName = "indonesian"
    else if (userLang === "fa") userLangName = "persian"
    else if (userLang === "ar") userLangName = "arabic"
    else if (userLang === "fr") userLangName = "french"
    else if (userLang === "es") userLangName = "spanish"
    else if (userLang === "de") userLangName = "german"
    else if (userLang === "ru") userLangName = "russian"
    else if (userLang === "hi") userLangName = "hindi"

    matched.sort(function(a, b) {
      var langA = String(a.language || "").toLowerCase()
      var langB = String(b.language || "").toLowerCase()

      var rankA = 2
      var rankB = 2
      if (userLangName && langA === userLangName) rankA = 0
      else if (langA === "english") rankA = 1

      if (userLangName && langB === userLangName) rankB = 0
      else if (langB === "english") rankB = 1

      if (rankA !== rankB) return rankA - rankB
      if (langA !== langB) return langA.localeCompare(langB)
      return String(a.name || "").localeCompare(String(b.name || ""))
    })
    return matched
  }

  readonly property var filteredWBWLanguages: {
    var q = peace.wbwSearchQuery.toLowerCase()
    var items = peace.allWBWLanguages || []
    var matched = []
    for (var i = 0; i < items.length; i++) {
      var item = items[i]
      if (!q) {
        matched.push(item)
        continue
      }
      var name = String(item.name || "").toLowerCase()
      var nativeName = String(item.native_name || "").toLowerCase()
      var code = String(item.iso_code || "").toLowerCase()
      if (name.indexOf(q) !== -1 || nativeName.indexOf(q) !== -1 || code.indexOf(q) !== -1) {
        matched.push(item)
      }
    }
    return matched
  }

  readonly property string selectedTranslationName: {
    for (var i = 0; i < peace.allTranslations.length; i++) {
      if (String(peace.allTranslations[i].id) === peace.selectedTranslation) {
        return peace.allTranslations[i].name
      }
    }
    return "ID " + peace.selectedTranslation
  }

  readonly property string selectedWBWLanguageName: {
    for (var i = 0; i < peace.allWBWLanguages.length; i++) {
      if (String(peace.allWBWLanguages[i].iso_code) === peace.selectedWBWLanguage) {
        return peace.allWBWLanguages[i].name
      }
    }
    return peace.selectedWBWLanguage.toUpperCase()
  }

  property string reciterID: "7"
  property var reciterOptions: [{
    "value": "7",
    "label": "Mishary Alafasy"
  }, {
    "value": "6",
    "label": "Mahmoud Al-Husary"
  }]
  property bool recitersLoaded: false
  property int bufferSize: 10
  property int leadSize: 2
  property int workerCount: 2
  readonly property color foreground: bar ? bar.foreground : Color.foreground
  readonly property string fontFamily: bar ? bar.fontFamily : Style.font.family
  readonly property string homeDirectory: String(Quickshell.env("HOME") || "")
  readonly property string currentBackgroundLink: homeDirectory + "/.local/state/omarchy/current/background"
  property string artworkPath: ""
  readonly property bool isPlaying: statusText.indexOf("playing") === 0
  readonly property bool isPaused: isPlaying && statusText.indexOf(", paused") !== -1
  readonly property bool isPreparing: statusText.indexOf("preparing") === 0
  readonly property bool selectedReferenceIsPlaying: {
    var match = peace.selectedReferenceMatch
    if (!peace.isPlaying || !match || Number(match[1]) !== peace.currentSurahNumber)
      return false
    if (!match[2])
      return true
    var start = Number(match[2])
    if (peace.referenceText.indexOf("-") === -1)
      return peace.currentAyahNumber === start
    var end = match[3] ? Number(match[3]) : 0
    return peace.currentAyahNumber >= start && (end === 0 || peace.currentAyahNumber <= end)
  }
  readonly property string playbackStatus: isPaused ? "Paused"
    : (isPlaying ? "Playing"
       : (isPreparing ? "Preparing"
          : (statusText === "Playback status unavailable" ? "Unavailable" : "Ready")))
  readonly property var surahNames: [
    "Al-Fatihah", "Al-Baqarah", "Ali 'Imran", "An-Nisa", "Al-Ma'idah", "Al-An'am",
    "Al-A'raf", "Al-Anfal", "At-Tawbah", "Yunus", "Hud", "Yusuf",
    "Ar-Ra'd", "Ibrahim", "Al-Hijr", "An-Nahl", "Al-Isra", "Al-Kahf",
    "Maryam", "Ta-Ha", "Al-Anbiya", "Al-Hajj", "Al-Mu'minun", "An-Nur",
    "Al-Furqan", "Ash-Shu'ara", "An-Naml", "Al-Qasas", "Al-Ankabut", "Ar-Rum",
    "Luqman", "As-Sajdah", "Al-Ahzab", "Saba", "Fatir", "Ya-Sin",
    "As-Saffat", "Sad", "Az-Zumar", "Ghafir", "Fussilat", "Ash-Shura",
    "Az-Zukhruf", "Ad-Dukhan", "Al-Jathiyah", "Al-Ahqaf", "Muhammad", "Al-Fath",
    "Al-Hujurat", "Qaf", "Adh-Dhariyat", "At-Tur", "An-Najm", "Al-Qamar",
    "Ar-Rahman", "Al-Waqi'ah", "Al-Hadid", "Al-Mujadila", "Al-Hashr", "Al-Mumtahanah",
    "As-Saff", "Al-Jumu'ah", "Al-Munafiqun", "At-Taghabun", "At-Talaq", "At-Tahrim",
    "Al-Mulk", "Al-Qalam", "Al-Haqqah", "Al-Ma'arij", "Nuh", "Al-Jinn",
    "Al-Muzzammil", "Al-Muddaththir", "Al-Qiyamah", "Al-Insan", "Al-Mursalat", "An-Naba",
    "An-Nazi'at", "Abasa", "At-Takwir", "Al-Infitar", "Al-Mutaffifin", "Al-Inshiqaq",
    "Al-Buruj", "At-Tariq", "Al-A'la", "Al-Ghashiyah", "Al-Fajr", "Al-Balad",
    "Ash-Shams", "Al-Layl", "Ad-Duha", "Ash-Sharh", "At-Tin", "Al-Alaq",
    "Al-Qadr", "Al-Bayyinah", "Az-Zalzalah", "Al-Adiyat", "Al-Qari'ah", "At-Takathur",
    "Al-Asr", "Al-Humazah", "Al-Fil", "Quraysh", "Al-Ma'un", "Al-Kawthar",
    "Al-Kafirun", "An-Nasr", "Al-Masad", "Al-Ikhlas", "Al-Falaq", "An-Nas"
  ]
  readonly property var selectedReferenceMatch: String(peace.referenceText || "").trim().match(/^([0-9]+)(?::([0-9]+)(?:-([0-9]*))?)?$/)
  readonly property var liveReferenceMatch: String(peace.statusText || "").match(/^playing ([0-9]+):([0-9]+)/)
  readonly property int currentSurahNumber: liveReferenceMatch ? Number(liveReferenceMatch[1]) : (selectedReferenceMatch ? Number(selectedReferenceMatch[1]) : 0)
  readonly property int currentAyahNumber: liveReferenceMatch ? Number(liveReferenceMatch[2]) : (selectedReferenceMatch && selectedReferenceMatch[2] ? Number(selectedReferenceMatch[2]) : 1)
  readonly property string currentSurahName: currentSurahNumber > 0 ? (surahNames[currentSurahNumber - 1] || "Surah " + currentSurahNumber) : "Choose a Surah"
  readonly property string reciterName: {
    for (var index = 0; index < peace.reciterOptions.length; index++) {
      if (String(peace.reciterOptions[index].value) === peace.reciterID)
        return peace.reciterOptions[index].label
    }
    return "Reciter " + peace.reciterID
  }

  function open() {
    peace.artworkCaptured = false
    peace.artworkIsSessionBase = false
    peace.refreshStatus()
    peace.refreshArtwork()
    peace.refreshReciters()
    peace.refreshTranslations()
    peace.refreshWBWLanguages()
    peace.loadSettings()
  }

  function closePickers() {
    peace.translationPickerOpen = false
    peace.wbwPickerOpen = false
  }

  function openTranslationPicker() {
    peace.wbwPickerOpen = false
    peace.translationPickerOpen = true
    if (peace.allTranslations.length === 0)
      peace.refreshTranslations()
  }

  function openWBWPicker() {
    peace.translationPickerOpen = false
    peace.wbwPickerOpen = true
    if (peace.allWBWLanguages.length === 0)
      peace.refreshWBWLanguages()
  }

  function playOrPause() {
    if (peace.selectedReferenceIsPlaying) {
      peace.runPeace(["pause"])
      statusRefreshTimer.restart()
      return
    }
    var reference = peace.referenceText.trim()
    var args = [
      "listen", reference, "--reciter", peace.reciterID,
      "--basmala=" + String(peace.basmalaEnabled)
    ].concat(peace.displayFlags(), peace.contentFlags())
    if (peace.useThemeColors)
      args.push("--use-theme")
    if (peace.followEnabled) {
      args.push("--follow", "--buffer", String(peace.bufferSize), "--lead", String(peace.leadSize), "--jobs", String(peace.workerCount))
      if (peace.randomBackground)
        args.push("--random")
    }
    peace.captureArtworkForListen(args)
  }

  function persistSettings(newTranslation, newWbwLang) {
    if (newTranslation !== undefined && newTranslation !== null && String(newTranslation) !== "") {
      peace.selectedTranslation = String(newTranslation)
    }
    if (newWbwLang !== undefined && newWbwLang !== null && String(newWbwLang) !== "") {
      peace.selectedWBWLanguage = String(newWbwLang)
    }
    if (peace.bar && peace.bar.shell) {
      var next = {}
      if (peace.settings) {
        for (var k in peace.settings) if (k !== "id") next[k] = peace.settings[k]
      }
      next.translation = peace.selectedTranslation
      next.wbwLanguage = peace.selectedWBWLanguage
      peace.bar.shell.updateEntryInline(peace.moduleName, next)
      if (peace.hostWidget) peace.hostWidget.settings = next
    }
    peace.runPeace(["settings"].concat(peace.contentFlags()))
    peace.reRenderWallpaper()
  }

  function contentFlags() {
    return [
      "--translation=" + peace.selectedTranslation,
      "--wbw-language=" + peace.selectedWBWLanguage
    ]
  }

  function displayFlags() {
    return [
      "--show-translation=" + String(peace.showTranslation),
      "--show-word-translation=" + String(peace.showWordTranslation),
      "--show-reference=" + String(peace.showReference)
    ]
  }

  function reRenderWallpaper() {
    peace.runPeace([
      "wallpaper", "current"
    ].concat(peace.contentFlags(), peace.displayFlags()))
    statusRefreshTimer.restart()
    artworkRefreshTimer.restart()
  }

  function nextWallpaper() {
    peace.runPeace([
      "wallpaper", "next"
    ].concat(peace.contentFlags(), peace.displayFlags()))
  }

  function prev() {
    peace.runPeace(["__track", "prev"])
  }

  function next() {
    peace.runPeace(["__track", "next"])
  }

  function stop() {
    peace.runPeace(["stop"])
    peace.statusText = "not playing"
  }

  function runPeace(args) {
    if (!peace.bar)
      return

    var quoted = args.map(function(value) {
      return Util.shellQuote(String(value))
    })
    peace.bar.run(Util.shellQuote(peace.peaceExecutable) + " " + quoted.join(" "))
  }

  function refreshTranslations() {
    if (!peace.translationsLoading && !translationsProcess.running) {
      peace.translationsLoading = true
      peace.translationsError = ""
      translationsProcess.running = true
    }
  }

  function refreshWBWLanguages() {
    if (!peace.wbwLanguagesLoading && !wbwLanguagesProcess.running) {
      peace.wbwLanguagesLoading = true
      peace.wbwLanguagesError = ""
      wbwLanguagesProcess.running = true
    }
  }

  function loadSettings() {
    if (!settingsLoadProcess.running)
      settingsLoadProcess.running = true
  }

  function refreshStatus() {
    if (!statusProcess.running)
      statusProcess.running = true
  }

  function refreshReciters() {
    if (!peace.recitersLoaded && !recitersProcess.running)
      recitersProcess.running = true
  }

  function loadReciterOptions(output) {
    var lines = String(output || "").split("\n")
    var options = []
    for (var index = 0; index < lines.length; index++) {
      var line = lines[index].replace(/\r$/, "")
      if (!line || line.indexOf("ID") === 0)
        continue
      var match = line.match(/^([0-9]+)\s{2,}(.+?)\s{2,}(\S+)$/)
      if (!match)
        continue
      options.push({
        "value": match[1],
        "label": match[2]
      })
    }
    var labelCounts = ({})
    for (var countIndex = 0; countIndex < options.length; countIndex++) {
      var label = options[countIndex].label
      labelCounts[label] = (labelCounts[label] || 0) + 1
    }
    for (var optionIndex = 0; optionIndex < options.length; optionIndex++) {
      if (labelCounts[options[optionIndex].label] > 1)
        options[optionIndex].label += " (" + options[optionIndex].value + ")"
    }
    if (options.length > 0) {
      peace.reciterOptions = options
      peace.recitersLoaded = true
    }
  }

  function loadTranslationsJson(text) {
    try {
      var data = JSON.parse(text)
      if (Array.isArray(data) && data.length > 0) {
        peace.allTranslations = data
        peace.translationsLoaded = true
        peace.translationsError = ""
      }
    } catch (e) {
      console.warn("Failed to parse translations JSON:", e)
      peace.translationsError = "Failed to parse translations"
    } finally {
      peace.translationsLoading = false
    }
  }

  function loadWBWLanguagesJson(text) {
    try {
      var data = JSON.parse(text)
      if (Array.isArray(data) && data.length > 0) {
        peace.allWBWLanguages = data
        peace.wbwLanguagesLoaded = true
        peace.wbwLanguagesError = ""
      }
    } catch (e) {
      console.warn("Failed to parse WBW languages JSON:", e)
      peace.wbwLanguagesError = "Failed to parse languages"
    } finally {
      peace.wbwLanguagesLoading = false
    }
  }

  function applyLoadedSettingsJson(text) {
    try {
      var data = JSON.parse(text)
      if (data && typeof data === "object") {
        if (data.translation && (!peace.settings || !peace.settings.translation)) {
          peace.selectedTranslation = String(data.translation)
        }
        if (data.wbw_language && (!peace.settings || !peace.settings.wbwLanguage)) {
          peace.selectedWBWLanguage = String(data.wbw_language)
        }
      }
    } catch (e) {
      // ignore
    }
  }

  function refreshArtwork() {
    if (!peace.artworkCaptured && !artworkPathProcess.running && !baseArtworkProcess.running)
      artworkPathProcess.running = true
  }

  function captureArtworkForListen(args) {
    peace.pendingListenArgs = args
    peace.artworkCaptured = false
    peace.artworkIsSessionBase = false
    peace.artworkCandidate = ""
    peace.refreshArtwork()
  }

  function finishArtworkCapture(path) {
    if (path !== "")
      peace.artworkPath = path
    peace.artworkCaptured = true
    if (peace.pendingListenArgs !== null) {
      var args = peace.pendingListenArgs
      peace.pendingListenArgs = null
      peace.artworkIsSessionBase = !peace.followEnabled
      peace.runPeace(args)
      statusTimer.start()
    }
  }

  Process {
    id: artworkPathProcess

    command: ["readlink", "-f", peace.currentBackgroundLink]

    stdout: StdioCollector {
      onStreamFinished: {
        peace.artworkCandidate = String(text || "").trim()
        if (peace.artworkCandidate !== "") {
          baseArtworkProcess.running = true
        } else {
          peace.finishArtworkCapture("")
        }
      }
    }
  }

  Process {
    id: baseArtworkProcess

    command: ["cat", peace.artworkCandidate + ".base"]
    stdout: StdioCollector {
      onStreamFinished: {
        var basePath = String(text || "").trim()
        if (basePath !== "" && peace.pendingListenArgs === null)
          peace.artworkIsSessionBase = true
        peace.finishArtworkCapture(basePath || peace.artworkCandidate)
      }
    }
    onExited: function(exitCode) {
      if (exitCode !== 0 && !peace.artworkCaptured)
        peace.finishArtworkCapture(peace.artworkCandidate)
    }
  }

  Process {
    id: sessionBaseArtworkProcess

    command: ["cat", peace.homeDirectory + "/.cache/peace/follow/base-source"]
    stdout: StdioCollector {
      onStreamFinished: {
        var path = String(text || "").trim()
        if (path !== "") {
          peace.artworkPath = path
          peace.artworkIsSessionBase = true
        }
      }
    }
  }

  Process {
    id: recitersProcess

    command: [peace.peaceExecutable, "reciters"]
    stdout: StdioCollector {
      onStreamFinished: peace.loadReciterOptions(text)
    }
    onExited: function(exitCode) {
      if (exitCode !== 0)
        console.warn("peace reciters failed with exit code", exitCode)
    }
  }

  Process {
    id: statusProcess

    command: [peace.peaceExecutable, "status"]
    onExited: function(exitCode) {
      if (exitCode !== 0 && peace.statusText === "Checking playback…")
        peace.statusText = "Playback status unavailable"
    }

    stdout: StdioCollector {
      onStreamFinished: {
        var value = String(text).trim()
        peace.statusText = value || "not playing"
        if (value.indexOf(", following") !== -1 && !peace.artworkIsSessionBase && !sessionBaseArtworkProcess.running)
          sessionBaseArtworkProcess.running = true
      }
    }
  }

  Process {
    id: translationsProcess

    command: [peace.peaceExecutable, "translations", "--json"]
    stdout: StdioCollector {
      onStreamFinished: peace.loadTranslationsJson(text)
    }
    onExited: function(exitCode) {
      peace.translationsLoading = false
      if (exitCode !== 0 && peace.allTranslations.length === 0)
        peace.translationsError = "Failed to load translations"
    }
  }

  Process {
    id: wbwLanguagesProcess

    command: [peace.peaceExecutable, "languages", "--json"]
    stdout: StdioCollector {
      onStreamFinished: peace.loadWBWLanguagesJson(text)
    }
    onExited: function(exitCode) {
      peace.wbwLanguagesLoading = false
      if (exitCode !== 0 && peace.allWBWLanguages.length === 0)
        peace.wbwLanguagesError = "Failed to load languages"
    }
  }

  Process {
    id: settingsLoadProcess

    command: [peace.peaceExecutable, "settings", "--json"]
    stdout: StdioCollector {
      onStreamFinished: peace.applyLoadedSettingsJson(text)
    }
  }

  Timer {
    interval: 3000
    repeat: true
    running: peace.active
    onTriggered: peace.refreshArtwork()
  }

  Timer {
    id: statusTimer

    interval: 1000
    repeat: true
    running: peace.active
    onTriggered: peace.refreshStatus()
  }

  Timer {
    id: statusRefreshTimer

    interval: 150
    onTriggered: peace.refreshStatus()
  }

  Timer {
    id: artworkRefreshTimer

    interval: 500
    onTriggered: {
      peace.artworkCaptured = false
      peace.artworkCandidate = ""
      peace.refreshArtwork()
    }
  }

  Component.onCompleted: {
    peace.refreshStatus()
    peace.refreshArtwork()
    peace.refreshReciters()
    peace.refreshTranslations()
    peace.refreshWBWLanguages()
    peace.loadSettings()
  }
}
