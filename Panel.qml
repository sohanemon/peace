import QtQuick
import QtQuick.Controls
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
  property string statusText: "Checking playback…"
  property string requestError: ""

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
    root.bar.run("peace " + quoted.join(" "))
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
    contentWidth: panel.fittedContentWidth(Style.space(380))
    contentHeight: panel.fittedContentHeight(content.implicitHeight)

    PanelKeyCatcher {
      id: keyCatcher
      anchors.fill: parent
      blocked: referenceInput.activeFocus
      onCloseRequested: root.close()

      Column {
        id: content
        width: parent.width
        spacing: Style.space(10)

        Text {
          text: "OMAPEACE"
          color: root.bar ? root.bar.foreground : Color.foreground
          font.family: root.bar ? root.bar.fontFamily : Style.font.family
          font.pixelSize: Style.font.heading
          font.bold: true
        }

        Text {
          width: parent.width
          text: root.statusText
          color: root.bar ? root.bar.foreground : Color.foreground
          font.family: root.bar ? root.bar.fontFamily : Style.font.family
          font.pixelSize: Style.font.body
          wrapMode: Text.WordWrap
        }

        TextField {
          id: referenceInput
          width: parent.width
          text: "55"
          placeholderText: "Surah, ayah, or range (for example 55:1-10)"
          selectByMouse: true
        }

        ComboBox {
          id: reciterInput
          width: parent.width
          model: ["ar.alafasy", "ar.husary"]
        }

        CheckBox {
          id: followInput
          text: "Follow ayah wallpaper"
          checked: false
        }

        Row {
          width: parent.width
          spacing: Style.space(8)
          visible: followInput.checked

          Text {
            anchors.verticalCenter: parent.verticalCenter
            text: "Buffer " + bufferInput.value
            color: root.bar ? root.bar.foreground : Color.foreground
          }

          Slider {
            id: bufferInput
            width: parent.width - Style.space(100)
            from: 1
            to: 20
            stepSize: 1
            value: 10
          }
        }

        Row {
          width: parent.width
          spacing: Style.space(8)
          visible: followInput.checked

          Text {
            anchors.verticalCenter: parent.verticalCenter
            text: "Lead " + leadInput.value
            color: root.bar ? root.bar.foreground : Color.foreground
          }

          Slider {
            id: leadInput
            width: parent.width - Style.space(100)
            from: 1
            to: 5
            stepSize: 1
            value: 2
          }
        }

        Row {
          width: parent.width
          spacing: Style.space(8)
          visible: followInput.checked

          Text {
            anchors.verticalCenter: parent.verticalCenter
            text: "Jobs " + jobsInput.value
            color: root.bar ? root.bar.foreground : Color.foreground
          }

          Slider {
            id: jobsInput
            width: parent.width - Style.space(100)
            from: 1
            to: 4
            stepSize: 1
            value: 2
          }
        }

        Text {
          width: parent.width
          visible: requestError !== ""
          text: root.requestError
          color: Color.urgent
          wrapMode: Text.WordWrap
        }

        Row {
          width: parent.width
          spacing: Style.space(8)

          Button {
            text: "Listen"
            onClicked: {
              var args = ["listen", referenceInput.text.trim(), "--reciter", reciterInput.currentText]
              if (followInput.checked) {
                args.push("--follow", "--buffer", String(bufferInput.value),
                  "--lead", String(leadInput.value), "--jobs", String(jobsInput.value))
              }
              root.requestError = ""
              root.runPeace(args)
              statusTimer.start()
            }
          }

          Button {
            text: "Pause"
            onClicked: root.runPeace(["pause"])
          }

          Button {
            text: "Stop"
            onClicked: {
              root.runPeace(["stop"])
              root.statusText = "not playing"
            }
          }
        }

        Button {
          text: "Wallpaper next"
          onClicked: root.runPeace(["wallpaper", "next"])
        }
      }
    }
  }

  Process {
    id: statusProcess
    command: ["peace", "status"]
    stdout: StdioCollector {
      onStreamFinished: {
        var value = String(text).trim()
        root.statusText = value || "not playing"
      }
    }
    onExited: function(exitCode) {
      if (exitCode !== 0 && root.statusText === "Checking playback…") root.statusText = "peace command unavailable"
    }
  }

  Timer {
    id: statusTimer
    interval: 1000
    repeat: true
    running: root.opened
    onTriggered: root.refreshStatus()
  }

  Component.onCompleted: root.refreshStatus()
}
