import QtQuick
import qs.Commons
import qs.Ui
import ".."

// Panel header: brand row on the left, playback status on the right.
Item {
  id: control

  property color foreground: Color.foreground
  property string fontFamily: Style.font.family
  property string title: "Peace"
  property string playbackStatus: ""
  property bool statusActive: false

  implicitHeight: brandRow.height

  Row {
    id: brandRow

    anchors.left: parent.left
    anchors.verticalCenter: parent.verticalCenter
    spacing: Style.space(14)

    SeljukStar {
      starSize: Style.space(52)
      iconColor: control.foreground
    }

    Text {
      anchors.verticalCenter: parent.verticalCenter
      text: control.title
      color: control.foreground
      font.family: control.fontFamily
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
      color: control.statusActive ? Color.accent : Qt.darker(control.foreground, 1.8)
    }

    Text {
      anchors.verticalCenter: parent.verticalCenter
      text: control.playbackStatus
      color: Qt.darker(control.foreground, 1.5)
      font.family: control.fontFamily
      font.pixelSize: Style.font.caption
    }
  }
}
