import QtQuick
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
  property string requestError: ""
  property bool basmalaEnabled: false
  property bool followEnabled: false
  property bool randomBackground: false
  property string reciterID: "ar.alafasy"
  property int bufferSize: 10
  property int leadSize: 2
  property int workerCount: 2

  readonly property color foreground: root.bar ? root.bar.foreground : Color.foreground
  readonly property string fontFamily: root.bar ? root.bar.fontFamily : Style.font.family
  readonly property bool isPlaying: statusText.indexOf("playing") === 0
  readonly property bool isPreparing: statusText.indexOf("preparing") === 0
  readonly property string playbackState: isPlaying ? "PLAYING" : (isPreparing ? "PREPARING" : "READY")

  function open() {
    root.controller.show()
    refreshStatus()
  }

  function close() {
    root.controller.hide()
  }

  function toggle() {
    if (root.opened) root.close()
    else root.open()
  }

  function closeForPopoutSwitch() {
    root.close()
  }

  function runPeace(args) {
    if (!root.bar) return
    var quoted = args.map(function(value) { return Util.shellQuote(String(value)) })
    root.bar.run(Util.shellQuote(root.peaceExecutable) + " " + quoted.join(" "))
  }

  function refreshStatus() {
    if (!statusProcess.running) statusProcess.running = true
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
      blocked: referenceInput.activeFocus || reciterInput.popupOpen || basmalaInput.activeFocus
        || followInput.activeFocus || randomBackgroundInput.activeFocus
      onCloseRequested: root.close()

      Column {
        id: content
        width: parent.width
        spacing: Style.space(14)

        PanelHero {
          width: parent.width
          title: "Peace"
          meta: "QURAN RECITATION"
          foreground: root.foreground
          fontFamily: root.fontFamily
          iconSize: Style.space(36)
          iconComponent: Component {
            Text {
              textFormat: Text.PlainText
              text: "󰝚"
              color: root.foreground
              font.family: root.fontFamily
              font.pixelSize: Style.font.display
            }
          }
        }

        Text {
          visible: root.requestError !== ""
          width: parent.width
          text: root.requestError
          color: Color.urgent
          font.family: root.fontFamily
          font.pixelSize: Style.font.bodySmall
          wrapMode: Text.WordWrap
        }

        PanelSeparator { foreground: root.foreground }

        Column {
          width: parent.width
          spacing: Style.space(8)

          PanelSectionHeader {
            text: "NOW PLAYING"
            foreground: root.foreground
            fontFamily: root.fontFamily
          }

          BorderSurface {
            width: parent.width
            implicitHeight: statusCardContent.implicitHeight + Style.space(20)
            radius: Style.cornerRadius
            color: "transparent"
            borderSpec: Border.controlSpec("normal", root.foreground, Color.accent)

            Row {
              id: statusCardContent
              anchors.fill: parent
              anchors.leftMargin: Style.space(14)
              anchors.rightMargin: Style.space(14)
              spacing: Style.space(12)

              Rectangle {
                width: Style.space(8)
                height: width
                radius: width / 2
                anchors.verticalCenter: parent.verticalCenter
                color: root.isPlaying || root.isPreparing ? Color.accent : Qt.darker(root.foreground, 1.6)
              }

              Column {
                width: parent.width - Style.space(20)
                anchors.verticalCenter: parent.verticalCenter
                spacing: Style.space(2)

                Text {
                  width: parent.width
                  text: root.statusText === "not playing" ? "Ready when you are" : root.statusText
                  color: root.foreground
                  font.family: root.fontFamily
                  font.pixelSize: Style.font.body
                  font.bold: true
                  elide: Text.ElideRight
                }

                Text {
                  width: parent.width
                  text: root.isPlaying ? "Recitation in progress" : (root.isPreparing ? "Preparing audio" : "Choose an ayah to begin")
                  color: Qt.darker(root.foreground, 1.5)
                  font.family: root.fontFamily
                  font.pixelSize: Style.font.bodySmall
                  elide: Text.ElideRight
                }
              }
            }
          }
        }

        PanelSeparator { foreground: root.foreground }

        Column {
          width: parent.width
          spacing: Style.space(8)

          PanelSectionHeader {
            text: "RECITATION"
            foreground: root.foreground
            fontFamily: root.fontFamily
          }

          Row {
            width: parent.width
            spacing: Style.space(10)

            TextField {
              id: referenceInput
              width: Style.space(142)
              text: "55"
              placeholderText: "55:1-10"
              selectByMouse: true
              font.pixelSize: Style.font.subtitle
            }

            Dropdown {
              id: reciterInput
              width: parent.width - referenceInput.width - parent.spacing
              showLabel: false
              value: root.reciterID
              options: [
                { value: "ar.alafasy", label: "Mishary Alafasy" },
                { value: "ar.husary", label: "Mahmoud Al-Husary" }
              ]
              onChanged: function(value) { root.reciterID = value }
            }
          }

          Toggle {
            id: basmalaInput
            width: parent.width
            label: "Basmala"
            description: "Play it, and display it when wallpaper follow is on"
            checked: root.basmalaEnabled
            onClicked: root.basmalaEnabled = !root.basmalaEnabled
          }

          Button {
            width: parent.width
            text: "Listen"
            selected: true
            onClicked: {
              var args = ["listen", referenceInput.text.trim(), "--reciter", root.reciterID]
              args.push("--basmala=" + String(basmalaInput.checked))
              if (followInput.checked) {
                args.push("--follow", "--buffer", String(root.bufferSize),
                  "--lead", String(root.leadSize), "--jobs", String(root.workerCount))
                if (randomBackgroundInput.checked) args.push("--random")
              }
              root.requestError = ""
              root.runPeace(args)
              statusTimer.start()
            }
          }

          Row {
            width: parent.width
            spacing: Style.space(6)

            Button {
              width: (parent.width - parent.spacing * 2) / 3
              text: "Pause"
              bordered: true
              onClicked: root.runPeace(["pause"])
            }

            Button {
              width: (parent.width - parent.spacing * 2) / 3
              text: "Stop"
              bordered: true
              onClicked: {
                root.runPeace(["stop"])
                root.statusText = "not playing"
              }
            }

            Button {
              width: (parent.width - parent.spacing * 2) / 3
              text: "Next wallpaper"
              bordered: true
              onClicked: root.runPeace(["wallpaper", "next"])
            }
          }
        }

        PanelSeparator { foreground: root.foreground }

        Column {
          width: parent.width
          spacing: Style.space(8)

          PanelSectionHeader {
            text: "WALLPAPER FOLLOW"
            foreground: root.foreground
            fontFamily: root.fontFamily
          }

          Toggle {
            id: followInput
            width: parent.width
            label: "Follow the recitation"
            description: "Set the wallpaper before each ayah"
            checked: root.followEnabled
            onClicked: root.followEnabled = !root.followEnabled
          }

          Toggle {
            id: randomBackgroundInput
            visible: followInput.checked
            width: parent.width
            label: "Shuffle backgrounds"
            description: "Choose from current theme and Peace images"
            checked: root.randomBackground
            onClicked: root.randomBackground = !root.randomBackground
          }
        }


      }
    }
  }

  Process {
    id: statusProcess
    command: [root.peaceExecutable, "status"]
    stdout: StdioCollector {
      onStreamFinished: {
        var value = String(text).trim()
        root.statusText = value || "not playing"
      }
    }
    onExited: function(exitCode) {
      if (exitCode !== 0 && root.statusText === "Checking playback…") root.statusText = "Playback status unavailable"
    }
  }

  Timer {
    id: statusTimer
    interval: 1000
    repeat: true
    running: root.opened
    onTriggered: root.refreshStatus()
  }

  Component.onCompleted: {
    root.refreshStatus()
  }
}
