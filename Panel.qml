import QtQuick
import QtQuick.Effects
import Quickshell
import Quickshell.Io
import qs.Commons
import qs.Ui

Panel {
  id: root
  moduleName: "io.github.sohanemon.omapeace"
  ipcTarget: "io.github.sohanemon.omapeace"
  manageIpc: false

  property var hostWidget: null
  property var anchorItem: null
  readonly property var barIdentity: hostWidget || root
  readonly property string peaceExecutable: decodeURIComponent(String(Qt.resolvedUrl("bin/peace")).replace(/^file:\/\//, ""))
  property string statusText: "Checking playback…"
  property bool artworkCaptured: false
  property bool artworkIsSessionBase: false
  property string artworkCandidate: ""
  property var pendingListenArgs: null
  property bool basmalaEnabled: false
  property bool followEnabled: false
  property bool randomBackground: false
  property bool advancedOptionsExpanded: false
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
    var q = translationSearchQuery.toLowerCase()
    var items = allTranslations || []
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

    var userLang = userLanguage
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
    var q = wbwSearchQuery.toLowerCase()
    var items = allWBWLanguages || []
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
    for (var i = 0; i < allTranslations.length; i++) {
      if (String(allTranslations[i].id) === selectedTranslation) {
        return allTranslations[i].name
      }
    }
    return "ID " + selectedTranslation
  }

  readonly property string selectedWBWLanguageName: {
    for (var i = 0; i < allWBWLanguages.length; i++) {
      if (String(allWBWLanguages[i].iso_code) === selectedWBWLanguage) {
        return allWBWLanguages[i].name
      }
    }
    return selectedWBWLanguage.toUpperCase()
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
  readonly property color foreground: root.bar ? root.bar.foreground : Color.foreground
  readonly property string fontFamily: root.bar ? root.bar.fontFamily : Style.font.family
  readonly property string homeDirectory: String(Quickshell.env("HOME") || "")
  readonly property string currentBackgroundLink: homeDirectory + "/.local/state/omarchy/current/background"
  property string artworkPath: ""
  readonly property bool isPlaying: statusText.indexOf("playing") === 0
  readonly property bool isPaused: isPlaying && statusText.indexOf(", paused") !== -1
  readonly property bool isPreparing: statusText.indexOf("preparing") === 0
  readonly property bool selectedReferenceIsPlaying: {
    var match = selectedReferenceMatch
    if (!isPlaying || !match || Number(match[1]) !== currentSurahNumber)
      return false
    if (!match[2])
      return true
    var start = Number(match[2])
    if (referenceInput.text.indexOf("-") === -1)
      return currentAyahNumber === start
    var end = match[3] ? Number(match[3]) : 0
    return currentAyahNumber >= start && (end === 0 || currentAyahNumber <= end)
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
  readonly property var selectedReferenceMatch: String(referenceInput.text || "").trim().match(/^([0-9]+)(?::([0-9]+)(?:-([0-9]*))?)?$/)
  readonly property var liveReferenceMatch: String(statusText || "").match(/^playing ([0-9]+):([0-9]+)/)
  readonly property int currentSurahNumber: liveReferenceMatch ? Number(liveReferenceMatch[1]) : (selectedReferenceMatch ? Number(selectedReferenceMatch[1]) : 0)
  readonly property int currentAyahNumber: liveReferenceMatch ? Number(liveReferenceMatch[2]) : (selectedReferenceMatch && selectedReferenceMatch[2] ? Number(selectedReferenceMatch[2]) : 1)
  readonly property string currentSurahName: currentSurahNumber > 0 ? (surahNames[currentSurahNumber - 1] || "Surah " + currentSurahNumber) : "Choose a Surah"
  readonly property string reciterName: {
    for (var index = 0; index < reciterOptions.length; index++) {
      if (String(reciterOptions[index].value) === reciterID)
        return reciterOptions[index].label
    }
    return "Reciter " + reciterID
  }

  function open() {
    root.controller.show()
    root.artworkCaptured = false
    root.artworkIsSessionBase = false
    refreshStatus()
    refreshArtwork()
    refreshReciters()
    refreshTranslations()
    refreshWBWLanguages()
    loadSettings()
  }

  function close() {
    root.closePickers()
    root.controller.hide()
  }

  function openTranslationPicker() {
    root.wbwPickerOpen = false
    root.translationPickerOpen = true
    root.translationSearchQuery = ""
    if (root.allTranslations.length === 0) {
      root.refreshTranslations()
    }
    Qt.callLater(function() {
      if (translationSearchField)
        translationSearchField.forceActiveFocus()
    })
  }

  function openWBWPicker() {
    root.translationPickerOpen = false
    root.wbwPickerOpen = true
    root.wbwSearchQuery = ""
    if (root.allWBWLanguages.length === 0) {
      root.refreshWBWLanguages()
    }
    Qt.callLater(function() {
      if (wbwSearchField)
        wbwSearchField.forceActiveFocus()
    })
  }

  function closePickers() {
    root.translationPickerOpen = false
    root.wbwPickerOpen = false
    keyCatcher.forceActiveFocus()
  }

  function persistSettings(newTranslation, newWbwLang) {
    if (newTranslation !== undefined && newTranslation !== null && String(newTranslation) !== "") {
      root.selectedTranslation = String(newTranslation)
    }
    if (newWbwLang !== undefined && newWbwLang !== null && String(newWbwLang) !== "") {
      root.selectedWBWLanguage = String(newWbwLang)
    }
    if (root.bar && root.bar.shell) {
      var next = {}
      if (root.settings) {
        for (var k in root.settings) if (k !== "id") next[k] = root.settings[k]
      }
      next.translation = root.selectedTranslation
      next.wbwLanguage = root.selectedWBWLanguage
      root.bar.shell.updateEntryInline(root.moduleName, next)
      if (root.hostWidget) root.hostWidget.settings = next
    }
    root.runPeace(["settings", "--translation=" + root.selectedTranslation, "--wbw-language=" + root.selectedWBWLanguage])
    root.reRenderWallpaper()
  }

  function reRenderWallpaper() {
    root.runPeace([
      "wallpaper", "current",
      "--translation=" + root.selectedTranslation,
      "--wbw-language=" + root.selectedWBWLanguage,
      "--show-translation=" + String(root.showTranslation),
      "--show-word-translation=" + String(root.showWordTranslation),
      "--show-reference=" + String(root.showReference)
    ])
    statusRefreshTimer.restart()
    artworkRefreshTimer.restart()
  }

  function refreshTranslations() {
    if (!root.translationsLoading && !translationsProcess.running) {
      root.translationsLoading = true
      root.translationsError = ""
      translationsProcess.running = true
    }
  }

  function refreshWBWLanguages() {
    if (!root.wbwLanguagesLoading && !wbwLanguagesProcess.running) {
      root.wbwLanguagesLoading = true
      root.wbwLanguagesError = ""
      wbwLanguagesProcess.running = true
    }
  }

  function loadSettings() {
    if (!settingsLoadProcess.running) {
      settingsLoadProcess.running = true
    }
  }

  function toggle() {
    if (root.opened)
      root.close()
    else
      root.open()
  }

  function closeForPopoutSwitch() {
    root.close()
  }

  function runPeace(args) {
    if (!root.bar)
      return

    var quoted = args.map(function(value) {
      return Util.shellQuote(String(value))
    })
    root.bar.run(Util.shellQuote(root.peaceExecutable) + " " + quoted.join(" "))
  }

  function refreshStatus() {
    if (!statusProcess.running)
      statusProcess.running = true

  }

  function refreshReciters() {
    if (!root.recitersLoaded && !recitersProcess.running)
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
      root.reciterOptions = options
      root.recitersLoaded = true
    }
  }

  function loadTranslationsJson(text) {
    try {
      var data = JSON.parse(text)
      if (Array.isArray(data) && data.length > 0) {
        root.allTranslations = data
        root.translationsLoaded = true
        root.translationsError = ""
      }
    } catch (e) {
      console.warn("Failed to parse translations JSON:", e)
      root.translationsError = "Failed to parse translations"
    } finally {
      root.translationsLoading = false
    }
  }

  function loadWBWLanguagesJson(text) {
    try {
      var data = JSON.parse(text)
      if (Array.isArray(data) && data.length > 0) {
        root.allWBWLanguages = data
        root.wbwLanguagesLoaded = true
        root.wbwLanguagesError = ""
      }
    } catch (e) {
      console.warn("Failed to parse WBW languages JSON:", e)
      root.wbwLanguagesError = "Failed to parse languages"
    } finally {
      root.wbwLanguagesLoading = false
    }
  }

  function applyLoadedSettingsJson(text) {
    try {
      var data = JSON.parse(text)
      if (data && typeof data === "object") {
        if (data.translation && (!root.settings || !root.settings.translation)) {
          root.selectedTranslation = String(data.translation)
        }
        if (data.wbw_language && (!root.settings || !root.settings.wbwLanguage)) {
          root.selectedWBWLanguage = String(data.wbw_language)
        }
      }
    } catch (e) {
      // ignore
    }
  }

  function refreshArtwork() {
    if (!artworkCaptured && !artworkPathProcess.running && !baseArtworkProcess.running)
      artworkPathProcess.running = true

  }

  function captureArtworkForListen(args) {
    root.pendingListenArgs = args
    root.artworkCaptured = false
    root.artworkIsSessionBase = false
    root.artworkCandidate = ""
    root.refreshArtwork()
  }

  function finishArtworkCapture(path) {
    if (path !== "")
      root.artworkPath = path
    root.artworkCaptured = true
    if (root.pendingListenArgs !== null) {
      var args = root.pendingListenArgs
      root.pendingListenArgs = null
      root.artworkIsSessionBase = !root.followEnabled
      root.runPeace(args)
      statusTimer.start()
    }
  }

  KeyboardPanel {
    id: panel

    anchorItem: root.anchorItem
    owner: root.barIdentity
    bar: root.bar
    open: root.opened
    focusTarget: keyCatcher
    contentWidth: panel.fittedContentWidth(Style.space(430))
    contentHeight: panel.fittedContentHeight(content.implicitHeight)

    PanelKeyCatcher {
      id: keyCatcher

      anchors.fill: parent
      blocked: referenceInput.activeFocus || reciterInput.popupOpen || basmalaToggle.activeFocus || followToggle.activeFocus || randomBackgroundToggle.activeFocus || translationToggle.activeFocus || wordTranslationToggle.activeFocus || referenceToggle.activeFocus || themeColorsToggle.activeFocus || translationGearBtn.activeFocus || wordTranslationGearBtn.activeFocus || root.translationPickerOpen || root.wbwPickerOpen
      onCloseRequested: {
        if (root.translationPickerOpen || root.wbwPickerOpen) {
          root.closePickers()
        } else {
          root.close()
        }
      }

      Column {
        id: content

        width: parent.width
        spacing: Style.space(12)

        Item {
          width: parent.width
          height: headerLeft.height

          Row {
            id: headerLeft

            anchors.left: parent.left
            anchors.verticalCenter: parent.verticalCenter
            spacing: Style.space(14)

            SeljukStar {
              starSize: Style.space(52)
              iconColor: root.foreground
            }

            Text {
              anchors.verticalCenter: parent.verticalCenter
              text: "Peace"
              color: root.foreground
              font.family: root.fontFamily
              font.pixelSize: Style.font.heading
              font.bold: true
            }

          }

          Row {
            anchors.right: parent.right
            anchors.verticalCenter: parent.verticalCenter
            spacing: Style.space(6)

            Rectangle {
              width: Style.space(5)
              height: width
              radius: width / 2
              anchors.verticalCenter: parent.verticalCenter
              color: root.isPlaying || root.isPreparing ? Color.accent : Qt.darker(root.foreground, 1.8)
            }

            Text {
              anchors.verticalCenter: parent.verticalCenter
              text: root.playbackStatus
              color: Qt.darker(root.foreground, 1.5)
              font.family: root.fontFamily
              font.pixelSize: Style.font.caption
            }

          }

        }

        PanelSeparator {
          foreground: root.foreground
        }

        Column {
          width: parent.width
          spacing: Style.space(10)

          Item {
            id: artworkFrame

            width: parent.width
            height: Style.space(168)

            Image {
              id: artworkImage

              anchors.fill: parent
              source: root.artworkPath !== "" ? Util.fileUrl(root.artworkPath) : Qt.resolvedUrl("assets/backgrounds/01-pattern.png")
              fillMode: Image.PreserveAspectCrop
              asynchronous: true
              cache: false
              smooth: true
              mipmap: true
              layer.enabled: true
              onStatusChanged: {
                if (status === Image.Error && root.artworkPath !== "")
                  root.artworkPath = ""

              }

              layer.effect: MultiEffect {
                maskEnabled: true
                maskSource: artworkMask
                maskThresholdMin: 0.5
                maskSpreadAtMin: 0.02
              }

            }

            Rectangle {
              id: artworkMask

              anchors.fill: parent
              radius: Style.cornerRadius
              color: "white"
              visible: false
              layer.enabled: true
            }

            Rectangle {
              anchors.fill: parent
              radius: Style.cornerRadius
              color: Qt.rgba(0, 0, 0, 0.46)
            }

            BorderSurface {
              anchors.fill: parent
              radius: Style.cornerRadius
              color: "transparent"
              borderSpec: Border.controlSpec("normal", root.foreground, Color.accent)
            }

            Row {
              anchors.centerIn: parent
              spacing: Style.space(18)

              Button {
                width: Style.space(42)
                iconText: "󰒮"
                iconSize: Style.font.icon
                tooltipText: "Previous ayah"
                Accessible.name: "Previous ayah"
                bordered: true
                enabled: root.isPlaying && !root.isPreparing
                anchors.verticalCenter: parent.verticalCenter
                onClicked: root.runPeace(["__track", "prev"])
              }

              Button {
                width: Style.space(58)
                height: width
                radius: width / 2
                iconText: root.isPlaying && !root.isPaused ? "󰏤" : "󰐊"
                iconSize: Style.font.iconLarge
                selected: true
                enabled: !root.isPreparing && root.pendingListenArgs === null
                tooltipText: root.isPlaying && !root.isPaused ? "Pause recitation" : (root.isPaused ? "Resume recitation" : "Play recitation")
                Accessible.name: root.isPlaying && !root.isPaused ? "Pause recitation" : (root.isPaused ? "Resume recitation" : "Play recitation")
                onClicked: {
                  var reference = referenceInput.text.trim()
                  if (root.selectedReferenceIsPlaying) {
                    root.runPeace(["pause"])
                    statusRefreshTimer.restart()
                    return
                  }
                  var args = ["listen", reference, "--reciter", root.reciterID]
                  args.push("--basmala=" + String(root.basmalaEnabled))
                  args.push("--show-translation=" + String(root.showTranslation))
                  args.push("--show-word-translation=" + String(root.showWordTranslation))
                  args.push("--show-reference=" + String(root.showReference))
                  args.push("--translation=" + root.selectedTranslation)
                  args.push("--wbw-language=" + root.selectedWBWLanguage)
                  if (root.useThemeColors)
                    args.push("--use-theme")
                  if (root.followEnabled) {
                    args.push("--follow", "--buffer", String(root.bufferSize), "--lead", String(root.leadSize), "--jobs", String(root.workerCount))
                    if (root.randomBackground)
                      args.push("--random")
                  }
                  root.captureArtworkForListen(args)
                }
              }

              Button {
                width: Style.space(42)
                iconText: "󰒭"
                iconSize: Style.font.icon
                tooltipText: "Next ayah"
                Accessible.name: "Next ayah"
                bordered: true
                enabled: root.isPlaying && !root.isPreparing
                anchors.verticalCenter: parent.verticalCenter
                onClicked: root.runPeace(["__track", "next"])
              }
            }

            Button {
              anchors.left: parent.left
              anchors.bottom: parent.bottom
              anchors.margins: Style.space(10)
              width: Style.space(36)
              iconText: "󰓛"
              iconSize: Style.font.icon
              tooltipText: "Stop recitation"
              Accessible.name: "Stop recitation"
              bordered: true
              enabled: root.isPlaying || root.isPreparing
              onClicked: {
                root.runPeace(["stop"])
                root.statusText = "not playing"
              }
            }

            Button {
              anchors.right: parent.right
              anchors.bottom: parent.bottom
              anchors.margins: Style.space(10)
              width: Style.space(36)
              iconText: "󰋩"
              iconSize: Style.font.icon
              tooltipText: "Show the next ayah wallpaper"
              Accessible.name: "Show the next ayah wallpaper"
              bordered: true
              onClicked: root.runPeace([
                "wallpaper", "next",
                "--translation=" + root.selectedTranslation,
                "--wbw-language=" + root.selectedWBWLanguage,
                "--show-translation=" + String(root.showTranslation),
                "--show-word-translation=" + String(root.showWordTranslation),
                "--show-reference=" + String(root.showReference)
              ])
            }

          }

          Row {
            width: parent.width
            spacing: Style.space(10)

            Column {
              width: parent.width - ayahBadge.width - parent.spacing
              anchors.verticalCenter: parent.verticalCenter
              spacing: Style.space(2)

              Text {
                width: parent.width
                text: root.currentSurahName
                color: root.foreground
                font.family: root.fontFamily
                font.pixelSize: Style.font.title
                font.bold: true
                elide: Text.ElideRight
              }

              Text {
                width: parent.width
                text: root.reciterName
                color: Qt.darker(root.foreground, 1.35)
                font.family: root.fontFamily
                font.pixelSize: Style.font.bodySmall
                elide: Text.ElideRight
              }

            }

            BorderSurface {
              id: ayahBadge

              implicitWidth: ayahText.implicitWidth + Style.space(18)
              implicitHeight: ayahText.implicitHeight + Style.space(10)
              anchors.verticalCenter: parent.verticalCenter
              radius: Style.cornerRadius
              color: Style.selectedFillFor(root.foreground, Color.accent)
              borderSpec: Border.controlSpec("selected", root.foreground, Color.accent)

              Text {
                id: ayahText

                anchors.centerIn: parent
                text: "Ayah " + root.currentAyahNumber
                color: root.foreground
                font.family: root.fontFamily
                font.pixelSize: Style.font.bodySmall
                font.bold: true
              }

            }

          }

          Column {
            width: parent.width
            spacing: Style.space(2)

            Row {
              width: parent.width
              height: referenceInput.height
              spacing: Style.spacing.sm

              TextField {
                id: referenceInput

                width: Style.space(84)
                height: Style.spacing.controlHeight
                text: "55"
                placeholderText: "55:1-10"
                selectByMouse: true
                horizontalAlignment: TextInput.AlignHCenter
                font.pixelSize: Style.font.body
                font.bold: true
                foreground: Qt.darker(root.foreground, 1.2)
                Accessible.name: "Surah and ayah reference"
              }

              Dropdown {
                id: reciterInput

                width: parent.width - referenceInput.width - parent.spacing
                rowHeight: referenceInput.height
                showLabel: false
                value: root.reciterID
                foreground: Qt.darker(root.foreground, 1.2)
                fontFamily: root.fontFamily
                Accessible.name: "Reciter"
                options: root.reciterOptions
                onChanged: function(value) {
                  root.reciterID = value
                }
              }

            }

            PanelSeparator {
              foreground: root.foreground
              strength: 0.1
            }

            Toggle {
              id: followToggle

              width: parent.width
              label: "Follow wallpaper"
              description: "Advance the wallpaper with each ayah"
              titleSize: Style.font.bodySmall
              checked: root.followEnabled
              color: followToggle.activeFocus ? Style.focusFillFor(root.foreground, Color.accent) : "transparent"
              borderSpec: Border.none()
              Accessible.name: "Follow wallpaper"
              onClicked: root.followEnabled = !root.followEnabled
            }

            PanelSeparator {
              foreground: root.foreground
              strength: 0.08
            }

            Item {
              id: advancedOptionsRow

              width: parent.width
              height: Style.space(38)
              Accessible.name: "Advanced options"
              Accessible.role: Accessible.Button

              Rectangle {
                anchors.fill: parent
                radius: Style.cornerRadius
                color: advancedOptionsMouse.containsMouse ? Style.hoverFillFor(root.foreground, Color.accent) : "transparent"

                Behavior on color {
                  ColorAnimation {
                    duration: 100
                  }

                }

              }

              Text {
                anchors.left: parent.left
                anchors.leftMargin: Style.spacing.rowPaddingX
                anchors.verticalCenter: parent.verticalCenter
                text: "Advanced options"
                color: root.foreground
                font.family: root.fontFamily
                font.pixelSize: Style.font.bodySmall
                font.bold: true
              }

              Text {
                anchors.right: parent.right
                anchors.rightMargin: Style.spacing.rowPaddingX
                anchors.verticalCenter: parent.verticalCenter
                text: "󰅂"
                color: Qt.darker(root.foreground, 1.3)
                font.family: root.fontFamily
                font.pixelSize: Style.font.body
                rotation: root.advancedOptionsExpanded ? 90 : 0

                Behavior on rotation {
                  NumberAnimation {
                    duration: 160
                    easing.type: Easing.OutCubic
                  }

                }

              }

              MouseArea {
                id: advancedOptionsMouse

                anchors.fill: parent
                hoverEnabled: true
                cursorShape: Qt.PointingHandCursor
                onClicked: root.advancedOptionsExpanded = !root.advancedOptionsExpanded
              }

            }

            Item {
              id: advancedOptionsBody

              width: parent.width
              height: root.advancedOptionsExpanded ? advancedOptionsContent.implicitHeight : 0
              implicitHeight: height
              clip: true
              opacity: root.advancedOptionsExpanded ? 1 : 0

              Column {
                id: advancedOptionsContent

                width: parent.width
                spacing: Style.space(2)

                Toggle {
                  id: basmalaToggle

                  width: parent.width
                  label: "Basmala"
                  description: "Open each Surah with Basmala"
                  titleSize: Style.font.bodySmall
                  checked: root.basmalaEnabled
                  color: basmalaToggle.activeFocus ? Style.focusFillFor(root.foreground, Color.accent) : "transparent"
                  borderSpec: Border.none()
                  Accessible.name: "Basmala introduction"
                  onClicked: root.basmalaEnabled = !root.basmalaEnabled
                }

                Toggle {
                  id: randomBackgroundToggle

                  visible: followToggle.checked
                  width: parent.width
                  label: "Shuffle backgrounds"
                  description: "Pick a random wallpaper per ayah"
                  titleSize: Style.font.bodySmall
                  checked: root.randomBackground
                  color: randomBackgroundToggle.activeFocus ? Style.focusFillFor(root.foreground, Color.accent) : "transparent"
                  borderSpec: Border.none()
                  Accessible.name: "Shuffle wallpaper backgrounds"
                  onClicked: root.randomBackground = !root.randomBackground
                }

                Row {
                  width: parent.width
                  spacing: Style.space(6)

                  Toggle {
                    id: translationToggle

                    width: parent.width - translationGearBtn.width - parent.spacing
                    label: "Show translation"
                    description: "Display the ayah translation"
                    titleSize: Style.font.bodySmall
                    checked: root.showTranslation
                    color: translationToggle.activeFocus ? Style.focusFillFor(root.foreground, Color.accent) : "transparent"
                    borderSpec: Border.none()
                    Accessible.name: "Show ayah translation"
                    onClicked: root.showTranslation = !root.showTranslation
                  }

                  Button {
                    id: translationGearBtn

                    width: Style.space(38)
                    height: Style.space(38)
                    anchors.verticalCenter: parent.verticalCenter
                    iconText: "󰒓"
                    iconSize: Style.font.icon
                    bordered: true
                    focusable: enabled
                    enabled: root.showTranslation
                    opacity: enabled ? 1.0 : 0.35
                    tooltipText: enabled ? "Choose translation (" + root.selectedTranslationName + ")" : "Enable translation to configure"
                    Accessible.name: "Choose translation"
                    onClicked: root.openTranslationPicker()
                  }
                }

                Row {
                  width: parent.width
                  spacing: Style.space(6)

                  Toggle {
                    id: wordTranslationToggle

                    width: parent.width - wordTranslationGearBtn.width - parent.spacing
                    label: "Show word meanings"
                    description: "Display word-by-word translations"
                    titleSize: Style.font.bodySmall
                    checked: root.showWordTranslation
                    color: wordTranslationToggle.activeFocus ? Style.focusFillFor(root.foreground, Color.accent) : "transparent"
                    borderSpec: Border.none()
                    Accessible.name: "Show word-by-word translations"
                    onClicked: root.showWordTranslation = !root.showWordTranslation
                  }

                  Button {
                    id: wordTranslationGearBtn

                    width: Style.space(38)
                    height: Style.space(38)
                    anchors.verticalCenter: parent.verticalCenter
                    iconText: "󰒓"
                    iconSize: Style.font.icon
                    bordered: true
                    focusable: enabled
                    enabled: root.showWordTranslation
                    opacity: enabled ? 1.0 : 0.35
                    tooltipText: enabled ? "Choose word-by-word language (" + root.selectedWBWLanguageName + ")" : "Enable word meanings to configure"
                    Accessible.name: "Choose word-by-word language"
                    onClicked: root.openWBWPicker()
                  }
                }

                Toggle {
                  id: referenceToggle

                  width: parent.width
                  label: "Show reference"
                  description: "Display the Surah and ayah number"
                  titleSize: Style.font.bodySmall
                  checked: root.showReference
                  color: referenceToggle.activeFocus ? Style.focusFillFor(root.foreground, Color.accent) : "transparent"
                  borderSpec: Border.none()
                  Accessible.name: "Show surah and ayah reference"
                  onClicked: root.showReference = !root.showReference
                }

                Toggle {
                  id: themeColorsToggle

                  width: parent.width
                  label: "Use theme colors"
                  description: "Tint the ayah text with the active theme"
                  titleSize: Style.font.bodySmall
                  checked: root.useThemeColors
                  color: themeColorsToggle.activeFocus ? Style.focusFillFor(root.foreground, Color.accent) : "transparent"
                  borderSpec: Border.none()
                  Accessible.name: "Use active theme text colors"
                  onClicked: root.useThemeColors = !root.useThemeColors
                }

              }

              Behavior on height {
                NumberAnimation {
                  duration: 180
                  easing.type: Easing.OutCubic
                }

              }

              Behavior on opacity {
                NumberAnimation {
                  duration: 140
                  easing.type: Easing.OutCubic
                }

              }

            }

          }

        }

      }

      Item {
        id: pickerOverlay

        anchors.fill: parent
        visible: root.translationPickerOpen || root.wbwPickerOpen
        z: 100

        Rectangle {
          anchors.fill: parent
          color: Qt.rgba(0, 0, 0, 0.72)
          radius: Style.cornerRadius

          MouseArea {
            anchors.fill: parent
            onClicked: root.closePickers()
          }
        }

        BorderSurface {
          id: translationPickerCard

          visible: root.translationPickerOpen
          anchors.fill: parent
          anchors.margins: Style.space(12)
          radius: Style.cornerRadius
          color: Commons.Color.popups.background
          borderSpec: Border.localOrSurfaceSpec("popups", "border", Commons.Color.popups.border, Commons.Color.popups.border, Style.normalBorderWidth)
          clip: true

          MouseArea {
            anchors.fill: parent
          }

          Item {
            anchors.fill: parent
            anchors.margins: Style.space(14)

            Row {
              id: translationHeaderRow

              anchors.top: parent.top
              anchors.left: parent.left
              anchors.right: parent.right
              height: Style.space(32)

              Column {
                width: parent.width - closeTranslationBtn.width - parent.spacing
                anchors.verticalCenter: parent.verticalCenter
                spacing: Style.space(2)

                Text {
                  width: parent.width
                  text: "Select Translation"
                  color: root.foreground
                  font.family: root.fontFamily
                  font.pixelSize: Style.font.heading
                  font.bold: true
                  elide: Text.ElideRight
                }

                Text {
                  width: parent.width
                  text: root.filteredTranslations.length + " available"
                  color: Qt.darker(root.foreground, 1.4)
                  font.family: root.fontFamily
                  font.pixelSize: Style.font.caption
                }
              }

              Button {
                id: closeTranslationBtn

                width: Style.space(32)
                height: Style.space(32)
                anchors.verticalCenter: parent.verticalCenter
                iconText: "󰅖"
                iconSize: Style.font.icon
                tooltipText: "Close (Esc)"
                Accessible.name: "Close translation picker"
                onClicked: root.closePickers()
              }
            }

            TextField {
              id: translationSearchField

              anchors.top: translationHeaderRow.bottom
              anchors.topMargin: Style.space(10)
              anchors.left: parent.left
              anchors.right: parent.right
              height: Style.spacing.controlHeight
              placeholderText: "Search name, author, or language..."
              foreground: root.foreground
              accent: Color.accent
              font.family: root.fontFamily
              font.pixelSize: Style.font.body
              selectByMouse: true
              text: root.translationSearchQuery
              onTextChanged: root.translationSearchQuery = text

              Keys.onPressed: function(event) {
                if (event.key === Qt.Key_Escape) {
                  root.closePickers()
                  event.accepted = true
                } else if (event.key === Qt.Key_Down) {
                  if (translationListView.count > 0) {
                    translationListView.currentIndex = 0
                    translationListView.forceActiveFocus()
                  }
                  event.accepted = true
                } else if (event.key === Qt.Key_Return || event.key === Qt.Key_Enter) {
                  if (translationListView.count > 0 && translationListView.currentIndex >= 0) {
                    var item = root.filteredTranslations[translationListView.currentIndex]
                    if (item) {
                      root.persistSettings(item.id, null)
                      root.closePickers()
                    }
                  }
                  event.accepted = true
                }
              }
            }

            PanelSeparator {
              id: translationSep

              anchors.top: translationSearchField.bottom
              anchors.topMargin: Style.space(10)
              anchors.left: parent.left
              anchors.right: parent.right
              foreground: root.foreground
              strength: 0.1
            }

            BorderSurface {
              id: translationErrorBanner

              visible: root.translationsError !== ""
              anchors.top: translationSep.bottom
              anchors.topMargin: Style.space(6)
              anchors.left: parent.left
              anchors.right: parent.right
              radius: Style.cornerRadius
              color: Qt.rgba(1, 0.2, 0.2, 0.15)
              borderSpec: Border.localOrSurfaceSpec("popups", "border", "#ff6b6b", "#ff6b6b", 1)
              implicitHeight: translationErrText.implicitHeight + Style.space(12)

              Text {
                id: translationErrText

                anchors.centerIn: parent
                text: root.translationsError + (root.allTranslations.length > 0 ? " (showing cached)" : "")
                color: "#ff6b6b"
                font.family: root.fontFamily
                font.pixelSize: Style.font.caption
              }
            }

            Column {
              anchors.centerIn: parent
              spacing: Style.space(8)
              visible: root.translationsLoading && root.allTranslations.length === 0

              Text {
                anchors.horizontalCenter: parent.horizontalCenter
                text: "󰑐"
                font.family: root.fontFamily
                font.pixelSize: Style.space(28)
                color: Color.accent
              }

              Text {
                anchors.horizontalCenter: parent.horizontalCenter
                text: "Loading translations…"
                font.family: root.fontFamily
                font.pixelSize: Style.font.bodySmall
                color: Qt.darker(root.foreground, 1.3)
              }
            }

            Text {
              anchors.centerIn: parent
              visible: !root.translationsLoading && root.filteredTranslations.length === 0 && root.translationsError === ""
              text: "No translations found matching \"" + root.translationSearchQuery + "\""
              font.family: root.fontFamily
              font.pixelSize: Style.font.bodySmall
              color: Qt.darker(root.foreground, 1.4)
              wrapMode: Text.WordWrap
              horizontalAlignment: Text.AlignHCenter
              width: parent.width - Style.space(32)
            }

            ListView {
              id: translationListView

              anchors.top: translationErrorBanner.visible ? translationErrorBanner.bottom : translationSep.bottom
              anchors.topMargin: translationErrorBanner.visible ? Style.space(6) : Style.space(10)
              anchors.left: parent.left
              anchors.right: parent.right
              anchors.bottom: parent.bottom
              clip: true
              spacing: Style.space(4)
              boundsBehavior: Flickable.StopAtBounds
              model: root.filteredTranslations

              Keys.priority: Keys.BeforeItem
              Keys.onPressed: function(event) {
                if (event.key === Qt.Key_Escape) {
                  root.closePickers()
                  event.accepted = true
                } else if (event.key === Qt.Key_Down || event.text === "j") {
                  if (currentIndex < count - 1)
                    currentIndex++
                  event.accepted = true
                } else if (event.key === Qt.Key_Up || event.text === "k") {
                  if (currentIndex <= 0) {
                    translationSearchField.forceActiveFocus()
                  } else {
                    currentIndex--
                  }
                  event.accepted = true
                } else if (event.key === Qt.Key_Return || event.key === Qt.Key_Enter) {
                  if (currentIndex >= 0 && currentIndex < count) {
                    var item = root.filteredTranslations[currentIndex]
                    if (item) {
                      root.persistSettings(item.id, null)
                      root.closePickers()
                    }
                  }
                  event.accepted = true
                }
              }

              delegate: BorderSurface {
                required property var modelData
                required property int index

                readonly property bool isSelected: String(modelData.id) === root.selectedTranslation
                readonly property bool isCurrent: index === translationListView.currentIndex

                width: translationListView.width
                height: Style.space(52)
                radius: Style.cornerRadius
                color: isSelected ? Style.selectedFillFor(root.foreground, Color.accent) : (isCurrent || rowMouse.containsMouse ? Style.hoverFillFor(root.foreground, Color.accent) : "transparent")
                borderSpec: isSelected ? Border.controlSpec("selected", root.foreground, Color.accent) : (isCurrent ? Border.controlSpec("focus", root.foreground, Color.accent) : Border.none())

                Row {
                  anchors.fill: parent
                  anchors.leftMargin: Style.space(10)
                  anchors.rightMargin: Style.space(10)
                  spacing: Style.space(8)

                  Column {
                    width: parent.width - badgeRow.width - parent.spacing
                    anchors.verticalCenter: parent.verticalCenter
                    spacing: Style.space(2)

                    Text {
                      width: parent.width
                      text: modelData.name || "Translation " + modelData.id
                      color: root.foreground
                      font.family: root.fontFamily
                      font.pixelSize: Style.font.bodySmall
                      font.bold: isSelected
                      elide: Text.ElideRight
                    }

                    Text {
                      width: parent.width
                      text: (modelData.author && modelData.author !== modelData.name ? modelData.author + " • " : "") + (modelData.language ? (modelData.language.charAt(0).toUpperCase() + modelData.language.slice(1)) : "")
                      color: Qt.darker(root.foreground, 1.45)
                      font.family: root.fontFamily
                      font.pixelSize: Style.font.caption
                      elide: Text.ElideRight
                    }
                  }

                  Row {
                    id: badgeRow

                    anchors.verticalCenter: parent.verticalCenter
                    spacing: Style.space(6)

                    BorderSurface {
                      implicitWidth: langTagText.implicitWidth + Style.space(10)
                      implicitHeight: langTagText.implicitHeight + Style.space(6)
                      anchors.verticalCenter: parent.verticalCenter
                      radius: Style.space(4)
                      color: Qt.rgba(root.foreground.r, root.foreground.g, root.foreground.b, 0.08)
                      borderSpec: Border.none()

                      Text {
                        id: langTagText

                        anchors.centerIn: parent
                        text: String(modelData.language || "").toUpperCase()
                        color: Qt.darker(root.foreground, 1.3)
                        font.family: root.fontFamily
                        font.pixelSize: Style.space(9)
                        font.bold: true
                      }
                    }

                    Text {
                      visible: isSelected
                      anchors.verticalCenter: parent.verticalCenter
                      text: "󰄬"
                      color: Color.accent
                      font.family: root.fontFamily
                      font.pixelSize: Style.font.icon
                    }
                  }
                }

                MouseArea {
                  id: rowMouse

                  anchors.fill: parent
                  hoverEnabled: true
                  cursorShape: Qt.PointingHandCursor
                  onPositionChanged: translationListView.currentIndex = parent.index
                  onClicked: {
                    root.persistSettings(modelData.id, null)
                    root.closePickers()
                  }
                }
              }
            }
          }
        }

        BorderSurface {
          id: wbwPickerCard

          visible: root.wbwPickerOpen
          anchors.fill: parent
          anchors.margins: Style.space(12)
          radius: Style.cornerRadius
          color: Commons.Color.popups.background
          borderSpec: Border.localOrSurfaceSpec("popups", "border", Commons.Color.popups.border, Commons.Color.popups.border, Style.normalBorderWidth)
          clip: true

          MouseArea {
            anchors.fill: parent
          }

          Item {
            anchors.fill: parent
            anchors.margins: Style.space(14)

            Row {
              id: wbwHeaderRow

              anchors.top: parent.top
              anchors.left: parent.left
              anchors.right: parent.right
              height: Style.space(32)

              Column {
                width: parent.width - closeWbwBtn.width - parent.spacing
                anchors.verticalCenter: parent.verticalCenter
                spacing: Style.space(2)

                Text {
                  width: parent.width
                  text: "Word-by-Word Language"
                  color: root.foreground
                  font.family: root.fontFamily
                  font.pixelSize: Style.font.heading
                  font.bold: true
                  elide: Text.ElideRight
                }

                Text {
                  width: parent.width
                  text: root.filteredWBWLanguages.length + " verified languages"
                  color: Qt.darker(root.foreground, 1.4)
                  font.family: root.fontFamily
                  font.pixelSize: Style.font.caption
                }
              }

              Button {
                id: closeWbwBtn

                width: Style.space(32)
                height: Style.space(32)
                anchors.verticalCenter: parent.verticalCenter
                iconText: "󰅖"
                iconSize: Style.font.icon
                tooltipText: "Close (Esc)"
                Accessible.name: "Close WBW language picker"
                onClicked: root.closePickers()
              }
            }

            TextField {
              id: wbwSearchField

              anchors.top: wbwHeaderRow.bottom
              anchors.topMargin: Style.space(10)
              anchors.left: parent.left
              anchors.right: parent.right
              height: Style.spacing.controlHeight
              placeholderText: "Search language name or code..."
              foreground: root.foreground
              accent: Color.accent
              font.family: root.fontFamily
              font.pixelSize: Style.font.body
              selectByMouse: true
              text: root.wbwSearchQuery
              onTextChanged: root.wbwSearchQuery = text

              Keys.onPressed: function(event) {
                if (event.key === Qt.Key_Escape) {
                  root.closePickers()
                  event.accepted = true
                } else if (event.key === Qt.Key_Down) {
                  if (wbwListView.count > 0) {
                    wbwListView.currentIndex = 0
                    wbwListView.forceActiveFocus()
                  }
                  event.accepted = true
                } else if (event.key === Qt.Key_Return || event.key === Qt.Key_Enter) {
                  if (wbwListView.count > 0 && wbwListView.currentIndex >= 0) {
                    var item = root.filteredWBWLanguages[wbwListView.currentIndex]
                    if (item) {
                      root.persistSettings(null, item.iso_code)
                      root.closePickers()
                    }
                  }
                  event.accepted = true
                }
              }
            }

            PanelSeparator {
              id: wbwSep

              anchors.top: wbwSearchField.bottom
              anchors.topMargin: Style.space(10)
              anchors.left: parent.left
              anchors.right: parent.right
              foreground: root.foreground
              strength: 0.1
            }

            BorderSurface {
              id: wbwErrorBanner

              visible: root.wbwLanguagesError !== ""
              anchors.top: wbwSep.bottom
              anchors.topMargin: Style.space(6)
              anchors.left: parent.left
              anchors.right: parent.right
              radius: Style.cornerRadius
              color: Qt.rgba(1, 0.2, 0.2, 0.15)
              borderSpec: Border.localOrSurfaceSpec("popups", "border", "#ff6b6b", "#ff6b6b", 1)
              implicitHeight: wbwErrText.implicitHeight + Style.space(12)

              Text {
                id: wbwErrText

                anchors.centerIn: parent
                text: root.wbwLanguagesError + (root.allWBWLanguages.length > 0 ? " (showing cached)" : "")
                color: "#ff6b6b"
                font.family: root.fontFamily
                font.pixelSize: Style.font.caption
              }
            }

            Column {
              anchors.centerIn: parent
              spacing: Style.space(8)
              visible: root.wbwLanguagesLoading && root.allWBWLanguages.length === 0

              Text {
                anchors.horizontalCenter: parent.horizontalCenter
                text: "󰑐"
                font.family: root.fontFamily
                font.pixelSize: Style.space(28)
                color: Color.accent
              }

              Text {
                anchors.horizontalCenter: parent.horizontalCenter
                text: "Loading languages…"
                font.family: root.fontFamily
                font.pixelSize: Style.font.bodySmall
                color: Qt.darker(root.foreground, 1.3)
              }
            }

            Text {
              anchors.centerIn: parent
              visible: !root.wbwLanguagesLoading && root.filteredWBWLanguages.length === 0 && root.wbwLanguagesError === ""
              text: "No languages found matching \"" + root.wbwSearchQuery + "\""
              font.family: root.fontFamily
              font.pixelSize: Style.font.bodySmall
              color: Qt.darker(root.foreground, 1.4)
              wrapMode: Text.WordWrap
              horizontalAlignment: Text.AlignHCenter
              width: parent.width - Style.space(32)
            }

            ListView {
              id: wbwListView

              anchors.top: wbwErrorBanner.visible ? wbwErrorBanner.bottom : wbwSep.bottom
              anchors.topMargin: wbwErrorBanner.visible ? Style.space(6) : Style.space(10)
              anchors.left: parent.left
              anchors.right: parent.right
              anchors.bottom: parent.bottom
              clip: true
              spacing: Style.space(4)
              boundsBehavior: Flickable.StopAtBounds
              model: root.filteredWBWLanguages

              Keys.priority: Keys.BeforeItem
              Keys.onPressed: function(event) {
                if (event.key === Qt.Key_Escape) {
                  root.closePickers()
                  event.accepted = true
                } else if (event.key === Qt.Key_Down || event.text === "j") {
                  if (currentIndex < count - 1)
                    currentIndex++
                  event.accepted = true
                } else if (event.key === Qt.Key_Up || event.text === "k") {
                  if (currentIndex <= 0) {
                    wbwSearchField.forceActiveFocus()
                  } else {
                    currentIndex--
                  }
                  event.accepted = true
                } else if (event.key === Qt.Key_Return || event.key === Qt.Key_Enter) {
                  if (currentIndex >= 0 && currentIndex < count) {
                    var item = root.filteredWBWLanguages[currentIndex]
                    if (item) {
                      root.persistSettings(null, item.iso_code)
                      root.closePickers()
                    }
                  }
                  event.accepted = true
                }
              }

              delegate: BorderSurface {
                required property var modelData
                required property int index

                readonly property bool isSelected: String(modelData.iso_code) === root.selectedWBWLanguage
                readonly property bool isCurrent: index === wbwListView.currentIndex

                width: wbwListView.width
                height: Style.space(48)
                radius: Style.cornerRadius
                color: isSelected ? Style.selectedFillFor(root.foreground, Color.accent) : (isCurrent || wbwRowMouse.containsMouse ? Style.hoverFillFor(root.foreground, Color.accent) : "transparent")
                borderSpec: isSelected ? Border.controlSpec("selected", root.foreground, Color.accent) : (isCurrent ? Border.controlSpec("focus", root.foreground, Color.accent) : Border.none())

                Row {
                  anchors.fill: parent
                  anchors.leftMargin: Style.space(10)
                  anchors.rightMargin: Style.space(10)
                  spacing: Style.space(8)

                  Column {
                    width: parent.width - wbwBadgeRow.width - parent.spacing
                    anchors.verticalCenter: parent.verticalCenter
                    spacing: Style.space(2)

                    Text {
                      width: parent.width
                      text: modelData.name || modelData.iso_code
                      color: root.foreground
                      font.family: root.fontFamily
                      font.pixelSize: Style.font.bodySmall
                      font.bold: isSelected
                      elide: Text.ElideRight
                    }

                    Text {
                      width: parent.width
                      text: (modelData.native_name && modelData.native_name !== modelData.name ? modelData.native_name + " • " : "") + "ISO: " + modelData.iso_code
                      color: Qt.darker(root.foreground, 1.45)
                      font.family: root.fontFamily
                      font.pixelSize: Style.font.caption
                      elide: Text.ElideRight
                    }
                  }

                  Row {
                    id: wbwBadgeRow

                    anchors.verticalCenter: parent.verticalCenter
                    spacing: Style.space(6)

                    BorderSurface {
                      implicitWidth: wbwIsoTagText.implicitWidth + Style.space(10)
                      implicitHeight: wbwIsoTagText.implicitHeight + Style.space(6)
                      anchors.verticalCenter: parent.verticalCenter
                      radius: Style.space(4)
                      color: Qt.rgba(root.foreground.r, root.foreground.g, root.foreground.b, 0.08)
                      borderSpec: Border.none()

                      Text {
                        id: wbwIsoTagText

                        anchors.centerIn: parent
                        text: String(modelData.iso_code || "").toUpperCase()
                        color: Qt.darker(root.foreground, 1.3)
                        font.family: root.fontFamily
                        font.pixelSize: Style.space(9)
                        font.bold: true
                      }
                    }

                    Text {
                      visible: isSelected
                      anchors.verticalCenter: parent.verticalCenter
                      text: "󰄬"
                      color: Color.accent
                      font.family: root.fontFamily
                      font.pixelSize: Style.font.icon
                    }
                  }
                }

                MouseArea {
                  id: wbwRowMouse

                  anchors.fill: parent
                  hoverEnabled: true
                  cursorShape: Qt.PointingHandCursor
                  onPositionChanged: wbwListView.currentIndex = parent.index
                  onClicked: {
                    root.persistSettings(null, modelData.iso_code)
                    root.closePickers()
                  }
                }
              }
            }
          }
        }
      }

    }

  }

  Process {
    id: artworkPathProcess

    command: ["readlink", "-f", root.currentBackgroundLink]

    stdout: StdioCollector {
      onStreamFinished: {
        root.artworkCandidate = String(text || "").trim()
        if (root.artworkCandidate !== "") {
          baseArtworkProcess.running = true
        } else {
          root.finishArtworkCapture("")
        }

      }
    }

  }

  Process {
    id: baseArtworkProcess

    command: ["cat", root.artworkCandidate + ".base"]
    stdout: StdioCollector {
      onStreamFinished: {
        var basePath = String(text || "").trim()
        if (basePath !== "" && root.pendingListenArgs === null)
          root.artworkIsSessionBase = true
        root.finishArtworkCapture(basePath || root.artworkCandidate)
      }
    }
    onExited: function(exitCode) {
      if (exitCode !== 0 && !root.artworkCaptured)
        root.finishArtworkCapture(root.artworkCandidate)
    }
  }

  Process {
    id: sessionBaseArtworkProcess

    command: ["cat", root.homeDirectory + "/.cache/peace/follow/base-source"]
    stdout: StdioCollector {
      onStreamFinished: {
        var path = String(text || "").trim()
        if (path !== "") {
          root.artworkPath = path
          root.artworkIsSessionBase = true
        }
      }
    }
  }

  Process {
    id: recitersProcess

    command: [root.peaceExecutable, "reciters"]
    stdout: StdioCollector {
      onStreamFinished: root.loadReciterOptions(text)
    }
    onExited: function(exitCode) {
      if (exitCode !== 0)
        console.warn("peace reciters failed with exit code", exitCode)
    }
  }

  Process {
    id: statusProcess

    command: [root.peaceExecutable, "status"]
    onExited: function(exitCode) {
      if (exitCode !== 0 && root.statusText === "Checking playback…")
        root.statusText = "Playback status unavailable"

    }

    stdout: StdioCollector {
      onStreamFinished: {
        var value = String(text).trim()
        root.statusText = value || "not playing"
        if (value.indexOf(", following") !== -1 && !root.artworkIsSessionBase && !sessionBaseArtworkProcess.running)
          sessionBaseArtworkProcess.running = true

      }
    }

  }

  Timer {
    interval: 3000
    repeat: true
    running: root.opened
    onTriggered: root.refreshArtwork()
  }

  Timer {
    id: statusTimer

    interval: 1000
    repeat: true
    running: root.opened
    onTriggered: root.refreshStatus()
  }

  Timer {
    id: statusRefreshTimer

    interval: 150
    onTriggered: root.refreshStatus()
  }

  Process {
    id: translationsProcess

    command: [root.peaceExecutable, "translations", "--json"]
    stdout: StdioCollector {
      onStreamFinished: root.loadTranslationsJson(text)
    }
    onExited: function(exitCode) {
      root.translationsLoading = false
      if (exitCode !== 0 && root.allTranslations.length === 0)
        root.translationsError = "Failed to load translations"
    }
  }

  Process {
    id: wbwLanguagesProcess

    command: [root.peaceExecutable, "languages", "--json"]
    stdout: StdioCollector {
      onStreamFinished: root.loadWBWLanguagesJson(text)
    }
    onExited: function(exitCode) {
      root.wbwLanguagesLoading = false
      if (exitCode !== 0 && root.allWBWLanguages.length === 0)
        root.wbwLanguagesError = "Failed to load languages"
    }
  }

  Process {
    id: settingsLoadProcess

    command: [root.peaceExecutable, "settings", "--json"]
    stdout: StdioCollector {
      onStreamFinished: root.applyLoadedSettingsJson(text)
    }
  }

  Timer {
    id: artworkRefreshTimer

    interval: 500
    onTriggered: {
      root.artworkCaptured = false
      root.artworkCandidate = ""
      root.refreshArtwork()
    }
  }

  Component.onCompleted: {
    root.refreshStatus()
    root.refreshArtwork()
    root.refreshReciters()
    root.refreshTranslations()
    root.refreshWBWLanguages()
    root.loadSettings()
  }
}
