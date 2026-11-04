import QtQuick
import qs.Commons
import qs.Ui

// Track info row: surah + reciter labels with an ayah number badge.
Item {
  id: control

  property color foreground: Color.foreground
  property string fontFamily: Style.font.family
  property string surahName: ""
  property string reciterName: ""
  property int ayahNumber: 1

  implicitHeight: infoRow.height

  Row {
    id: infoRow

    width: parent.width
    spacing: Style.space(10)

    Column {
      width: parent.width - ayahBadge.width - parent.spacing
      anchors.verticalCenter: parent.verticalCenter
      spacing: Style.space(2)

      Text {
        width: parent.width
        text: control.surahName
        color: control.foreground
        font.family: control.fontFamily
        font.pixelSize: Style.font.title
        font.bold: true
        elide: Text.ElideRight
      }

      Text {
        width: parent.width
        text: control.reciterName
        color: Qt.darker(control.foreground, 1.35)
        font.family: control.fontFamily
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
      color: Style.selectedFillFor(control.foreground, Color.accent)
      borderSpec: Border.controlSpec("selected", control.foreground, Color.accent)

      Text {
        id: ayahText

        anchors.centerIn: parent
        text: "Ayah " + control.ayahNumber
        color: control.foreground
        font.family: control.fontFamily
        font.pixelSize: Style.font.bodySmall
        font.bold: true
      }
    }
  }
}
