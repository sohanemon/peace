import QtQuick
import QtQuick.Effects
import qs.Commons
import qs.Ui

// Playback artwork card: masked artwork image with transport controls.
Item {
  id: control

  property color foreground: Color.foreground
  property string fontFamily: Style.font.family
  property string artworkPath: ""
  property url fallbackArtwork: ""
  property bool isPlaying: false
  property bool isPaused: false
  property bool isPreparing: false
  property bool busy: false

  signal prevRequested()
  signal nextRequested()
  signal playPauseRequested()
  signal stopRequested()
  signal wallpaperNextRequested()
  signal artworkLoadFailed()

  Image {
    id: artworkImage

    anchors.fill: parent
    source: control.artworkPath !== "" ? Util.fileUrl(control.artworkPath) : control.fallbackArtwork
    fillMode: Image.PreserveAspectCrop
    asynchronous: true
    cache: false
    smooth: true
    mipmap: true
    layer.enabled: true
    onStatusChanged: {
      if (status === Image.Error && control.artworkPath !== "")
        control.artworkLoadFailed()
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
    borderSpec: Border.controlSpec("normal", control.foreground, Color.accent)
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
      enabled: control.isPlaying && !control.isPreparing
      anchors.verticalCenter: parent.verticalCenter
      onClicked: control.prevRequested()
    }

    Button {
      width: Style.space(58)
      height: width
      radius: width / 2
      iconText: control.isPlaying && !control.isPaused ? "󰏤" : "󰐊"
      iconSize: Style.font.iconLarge
      selected: true
      enabled: !control.isPreparing && !control.busy
      tooltipText: control.isPlaying && !control.isPaused ? "Pause recitation" : (control.isPaused ? "Resume recitation" : "Play recitation")
      Accessible.name: control.isPlaying && !control.isPaused ? "Pause recitation" : (control.isPaused ? "Resume recitation" : "Play recitation")
      onClicked: control.playPauseRequested()
    }

    Button {
      width: Style.space(42)
      iconText: "󰒭"
      iconSize: Style.font.icon
      tooltipText: "Next ayah"
      Accessible.name: "Next ayah"
      bordered: true
      enabled: control.isPlaying && !control.isPreparing
      anchors.verticalCenter: parent.verticalCenter
      onClicked: control.nextRequested()
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
    enabled: control.isPlaying || control.isPreparing
    onClicked: control.stopRequested()
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
    onClicked: control.wallpaperNextRequested()
  }
}
