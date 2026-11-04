import QtQuick
import qs.Commons
import qs.Ui

// Recitation selection: surah/ayah reference plus reciter picker.
Column {
  id: control

  property color foreground: Color.foreground
  property string fontFamily: Style.font.family
  property string reciterId: "7"
  property var reciterOptions: []

  property alias referenceText: referenceInput.text
  signal referenceEdited(string text)

  // INFO: mirrors keyCatcher blocked inputs from the monolithic panel
  readonly property bool blocksKeys: referenceInput.activeFocus
    || reciterInput.popupOpen

  signal reciterSelected(string value)

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
      foreground: Qt.darker(control.foreground, 1.2)
      Accessible.name: "Surah and ayah reference"
      onTextChanged: control.referenceEdited(text)
    }

    Dropdown {
      id: reciterInput

      width: parent.width - referenceInput.width - parent.spacing
      rowHeight: referenceInput.height
      showLabel: false
      value: control.reciterId
      foreground: Qt.darker(control.foreground, 1.2)
      fontFamily: control.fontFamily
      Accessible.name: "Reciter"
      options: control.reciterOptions
      onChanged: function(value) {
        control.reciterSelected(value)
      }
    }
  }
}
