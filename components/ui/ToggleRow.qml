import QtQuick
import qs.Commons
import qs.Ui

// REUSABLE: toggle row with an optional gear button on the right.
Row {
  id: control

  property string label: ""
  property string description: ""
  property bool checked: false
  property color foreground: Color.foreground
  property bool gearEnabled: false
  property string gearTooltip: ""
  property string gearName: ""

  // INFO: aggregated for keyCatcher blocked state in consumers
  readonly property bool hasKeyFocus: toggleItem.activeFocus || gearButton.activeFocus

  signal toggled()
  signal gearClicked()

  spacing: Style.space(6)

  Toggle {
    id: toggleItem

    width: parent.width - gearButton.width - parent.spacing
    label: control.label
    description: control.description
    titleSize: Style.font.bodySmall
    checked: control.checked
    color: activeFocus ? Style.focusFillFor(control.foreground, Color.accent) : "transparent"
    borderSpec: Border.none()
    Accessible.name: control.label
    onClicked: control.toggled()
  }

  Button {
    id: gearButton

    width: Style.space(38)
    height: Style.space(38)
    anchors.verticalCenter: parent.verticalCenter
    iconText: "󰒓"
    iconSize: Style.font.icon
    bordered: true
    focusable: enabled
    enabled: control.gearEnabled
    opacity: enabled ? 1.0 : 0.35
    tooltipText: control.gearTooltip
    Accessible.name: control.gearName
    onClicked: control.gearClicked()
  }
}
