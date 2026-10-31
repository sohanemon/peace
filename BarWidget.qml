import QtQuick
import qs.Ui

BarWidget {
  id: root
  moduleName: "io.github.sohanemon.omapeace"

  readonly property Item button: buttonItem
  readonly property bool opened: panelLoader.item ? panelLoader.item.opened === true : false
  readonly property bool popoutSwitchClosing: panelLoader.item ? panelLoader.item.popoutSwitchClosing === true : false

  function injectPanel() {
    if (!panelLoader.item) return
    panelLoader.item.hostWidget = root
    panelLoader.item.anchorItem = buttonItem
    panelLoader.item.bar = root.bar
  }

  function open() {
    if (panelLoader.item && panelLoader.item.open) panelLoader.item.open()
  }

  function close() {
    if (panelLoader.item && panelLoader.item.close) panelLoader.item.close()
  }

  function toggle() {
    if (panelLoader.item && panelLoader.item.toggle) panelLoader.item.toggle()
  }

  function closeForPopoutSwitch() {
    if (panelLoader.item && panelLoader.item.closeForPopoutSwitch) panelLoader.item.closeForPopoutSwitch()
  }

  implicitWidth: buttonItem.implicitWidth
  implicitHeight: barSize
  onBarChanged: injectPanel()

  WidgetButton {
    id: buttonItem
    anchors.fill: parent
    bar: root.bar
    text: "۞"
    tooltipText: "Omapeace listening controls"
    onPressed: function(buttonCode) {
      if (buttonCode === Qt.LeftButton) root.toggle()
    }
  }

  Loader {
    id: panelLoader
    active: true
    source: Qt.resolvedUrl("Panel.qml")
    visible: false
    onLoaded: {
      root.injectPanel()
      Qt.callLater(root.injectPanel)
    }
  }
}
