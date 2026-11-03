import QtQuick
import qs.Commons
import qs.Ui
import "ui"

// Recitation controls: reference/reciter row, follow toggle,
// collapsible advanced options section.
Column {
  id: control

  property color foreground: Color.foreground
  property string fontFamily: Style.font.family
  property string reciterId: "7"
  property var reciterOptions: []
  property bool followEnabled: false
  property bool basmalaEnabled: false
  property bool randomBackground: false
  property bool showTranslation: true
  property bool showWordTranslation: false
  property bool showReference: true
  property bool useThemeColors: false
  property string selectedTranslationName: ""
  property string selectedWBWLanguageName: ""

  property alias referenceText: referenceInput.text
  signal referenceEdited(string text)

  // INFO: mirrors keyCatcher blocked inputs from the monolithic panel
  readonly property bool blocksKeys: referenceInput.activeFocus
    || reciterInput.popupOpen
    || followToggle.activeFocus
    || basmalaToggle.activeFocus
    || randomBackgroundToggle.activeFocus
    || referenceToggle.activeFocus
    || themeColorsToggle.activeFocus
    || translationRow.hasKeyFocus
    || wordTranslationRow.hasKeyFocus

  // INFO: internal expander state, no external consumer
  property bool expanded: false

  signal reciterSelected(string value)
  signal followToggled()
  signal basmalaToggled()
  signal randomToggled()
  signal showTranslationToggled()
  signal showWordTranslationToggled()
  signal showReferenceToggled()
  signal useThemeColorsToggled()
  signal translationGearRequested()
  signal wbwGearRequested()

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

  PanelSeparator {
    foreground: control.foreground
    strength: 0.1
  }

  Toggle {
    id: followToggle

    width: parent.width
    label: "Follow wallpaper"
    description: "Advance the wallpaper with each ayah"
    titleSize: Style.font.bodySmall
    checked: control.followEnabled
    color: followToggle.activeFocus ? Style.focusFillFor(control.foreground, Color.accent) : "transparent"
    borderSpec: Border.none()
    Accessible.name: "Follow wallpaper"
    onClicked: control.followToggled()
  }

  PanelSeparator {
    foreground: control.foreground
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
      color: advancedOptionsMouse.containsMouse ? Style.hoverFillFor(control.foreground, Color.accent) : "transparent"

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
      color: control.foreground
      font.family: control.fontFamily
      font.pixelSize: Style.font.bodySmall
      font.bold: true
    }

    Text {
      anchors.right: parent.right
      anchors.rightMargin: Style.spacing.rowPaddingX
      anchors.verticalCenter: parent.verticalCenter
      text: "󰅂"
      color: Qt.darker(control.foreground, 1.3)
      font.family: control.fontFamily
      font.pixelSize: Style.font.body
      rotation: control.expanded ? 90 : 0

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
      onClicked: control.expanded = !control.expanded
    }
  }

  Item {
    id: advancedOptionsBody

    width: parent.width
    height: control.expanded ? advancedOptionsContent.implicitHeight : 0
    implicitHeight: height
    clip: true
    opacity: control.expanded ? 1 : 0

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
        checked: control.basmalaEnabled
        color: basmalaToggle.activeFocus ? Style.focusFillFor(control.foreground, Color.accent) : "transparent"
        borderSpec: Border.none()
        Accessible.name: "Basmala introduction"
        onClicked: control.basmalaToggled()
      }

      Toggle {
        id: randomBackgroundToggle

        visible: control.followEnabled
        width: parent.width
        label: "Shuffle backgrounds"
        description: "Pick a random wallpaper per ayah"
        titleSize: Style.font.bodySmall
        checked: control.randomBackground
        color: randomBackgroundToggle.activeFocus ? Style.focusFillFor(control.foreground, Color.accent) : "transparent"
        borderSpec: Border.none()
        Accessible.name: "Shuffle wallpaper backgrounds"
        onClicked: control.randomToggled()
      }

      ToggleRow {
        id: translationRow

        width: parent.width
        label: "Show translation"
        description: "Display the ayah translation"
        checked: control.showTranslation
        foreground: control.foreground
        gearEnabled: control.showTranslation
        gearTooltip: control.showTranslation ? "Choose translation (" + control.selectedTranslationName + ")" : "Enable translation to configure"
        gearName: "Choose translation"
        onToggled: control.showTranslationToggled()
        onGearClicked: control.translationGearRequested()
      }

      ToggleRow {
        id: wordTranslationRow

        width: parent.width
        label: "Show word meanings"
        description: "Display word-by-word translations"
        checked: control.showWordTranslation
        foreground: control.foreground
        gearEnabled: control.showWordTranslation
        gearTooltip: control.showWordTranslation ? "Choose word-by-word language (" + control.selectedWBWLanguageName + ")" : "Enable word meanings to configure"
        gearName: "Choose word-by-word language"
        onToggled: control.showWordTranslationToggled()
        onGearClicked: control.wbwGearRequested()
      }

      Toggle {
        id: referenceToggle

        width: parent.width
        label: "Show reference"
        description: "Display the Surah and ayah number"
        titleSize: Style.font.bodySmall
        checked: control.showReference
        color: referenceToggle.activeFocus ? Style.focusFillFor(control.foreground, Color.accent) : "transparent"
        borderSpec: Border.none()
        Accessible.name: "Show surah and ayah reference"
        onClicked: control.showReferenceToggled()
      }

      Toggle {
        id: themeColorsToggle

        width: parent.width
        label: "Use theme colors"
        description: "Tint the ayah text with the active theme"
        titleSize: Style.font.bodySmall
        checked: control.useThemeColors
        color: themeColorsToggle.activeFocus ? Style.focusFillFor(control.foreground, Color.accent) : "transparent"
        borderSpec: Border.none()
        Accessible.name: "Use active theme text colors"
        onClicked: control.useThemeColorsToggled()
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
