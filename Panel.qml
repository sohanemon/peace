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
  property bool showReference: true
  property bool useThemeColors: false
  property string reciterID: "ar.alafasy"
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
  readonly property string reciterName: reciterID === "ar.husary" ? "Mahmoud Al-Husary" : "Mishary Alafasy"

  function open() {
    root.controller.show()
    root.artworkCaptured = false
    root.artworkIsSessionBase = false
    refreshStatus()
    refreshArtwork()
  }

  function close() {
    root.controller.hide()
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
      blocked: referenceInput.activeFocus || reciterInput.popupOpen || basmalaToggle.activeFocus || followToggle.activeFocus || randomBackgroundToggle.activeFocus || translationToggle.activeFocus || referenceToggle.activeFocus || themeColorsToggle.activeFocus
      onCloseRequested: root.close()

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
                  args.push("--show-reference=" + String(root.showReference))
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
              onClicked: root.runPeace(["wallpaper", "next"])
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
                options: [{
                  "value": "ar.alafasy",
                  "label": "Mishary Alafasy"
                }, {
                  "value": "ar.husary",
                  "label": "Mahmoud Al-Husary"
                }]
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
              implicitHeight: Style.space(38)
              label: "Follow wallpaper"
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
                  implicitHeight: Style.space(38)
                  label: "Basmala"
                  description: ""
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
                  implicitHeight: Style.space(38)
                  label: "Shuffle backgrounds"
                  description: ""
                  titleSize: Style.font.bodySmall
                  checked: root.randomBackground
                  color: randomBackgroundToggle.activeFocus ? Style.focusFillFor(root.foreground, Color.accent) : "transparent"
                  borderSpec: Border.none()
                  Accessible.name: "Shuffle wallpaper backgrounds"
                  onClicked: root.randomBackground = !root.randomBackground
                }

                Toggle {
                  id: translationToggle

                  width: parent.width
                  implicitHeight: Style.space(38)
                  label: "Show translation"
                  description: ""
                  titleSize: Style.font.bodySmall
                  checked: root.showTranslation
                  color: translationToggle.activeFocus ? Style.focusFillFor(root.foreground, Color.accent) : "transparent"
                  borderSpec: Border.none()
                  Accessible.name: "Show ayah translation"
                  onClicked: root.showTranslation = !root.showTranslation
                }

                Toggle {
                  id: referenceToggle

                  width: parent.width
                  implicitHeight: Style.space(38)
                  label: "Show reference"
                  description: ""
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
                  implicitHeight: Style.space(38)
                  label: "Use theme colors"
                  description: ""
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

  Component.onCompleted: {
    root.refreshStatus()
    root.refreshArtwork()
  }
}
