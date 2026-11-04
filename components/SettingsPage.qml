import QtQuick
import qs.Commons
import qs.Ui
import "ui"

// Settings page shown inside the panel in place of the playback view.
// Owns every recitation/content preference; the panel provides state and IO.
Column {
  id: control

  property color foreground: Color.foreground
  property string fontFamily: Style.font.family
  property bool followEnabled: false
  property bool basmalaEnabled: false
  property bool randomBackground: false
  property bool showTranslation: true
  property bool showWordTranslation: false
  property bool showReference: true
  property bool useThemeColors: false
  property string selectedTranslationName: ""
  property string selectedWBWLanguageName: ""

  // INFO: aggregated for keyCatcher blocked state in the panel
  readonly property bool blocksKeys: followToggle.activeFocus
    || basmalaToggle.activeFocus
    || shuffleToggle.activeFocus
    || referenceToggle.activeFocus
    || themeColorsToggle.activeFocus
    || translationRow.hasKeyFocus
    || wordTranslationRow.hasKeyFocus

  signal closeRequested()
  signal followToggled()
  signal basmalaToggled()
  signal randomToggled()
  signal showTranslationToggled()
  signal showWordTranslationToggled()
  signal showReferenceToggled()
  signal useThemeColorsToggled()
  signal translationGearRequested()
  signal wbwGearRequested()

  width: parent.width
  spacing: Style.space(10)

  Item {
    width: parent.width
    height: Style.spacing.controlHeight

    Button {
      id: backButton

      width: Style.spacing.controlHeight
      height: Style.spacing.controlHeight
      anchors.left: parent.left
      anchors.verticalCenter: parent.verticalCenter
      iconText: "󰁍"
      iconSize: Style.font.icon
      tooltipText: "Back (Esc)"
      Accessible.name: "Back to playback"
      onClicked: control.closeRequested()
    }

    Text {
      anchors.left: backButton.right
      anchors.leftMargin: Style.spacing.sm
      anchors.right: parent.right
      anchors.verticalCenter: parent.verticalCenter
      text: "Settings"
      color: control.foreground
      font.family: control.fontFamily
      font.pixelSize: Style.font.heading
      font.bold: true
      elide: Text.ElideRight
    }
  }

  PanelSeparator {
    foreground: control.foreground
  }

  PanelSectionHeader {
    text: "Playback"
    foreground: control.foreground
    fontFamily: control.fontFamily
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

  Toggle {
    id: shuffleToggle

    width: parent.width
    enabled: control.followEnabled
    opacity: enabled ? 1 : 0.4
    label: "Shuffle backgrounds"
    description: control.followEnabled ? "Pick a random wallpaper per ayah" : "Turn on Follow wallpaper to use"
    titleSize: Style.font.bodySmall
    checked: control.randomBackground
    color: shuffleToggle.activeFocus ? Style.focusFillFor(control.foreground, Color.accent) : "transparent"
    borderSpec: Border.none()
    Accessible.name: "Shuffle wallpaper backgrounds"
    onClicked: {
      if (control.followEnabled)
        control.randomToggled()
    }
  }

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

  PanelSeparator {
    foreground: control.foreground
    strength: 0.08
  }

  PanelSectionHeader {
    text: "Content"
    foreground: control.foreground
    fontFamily: control.fontFamily
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
