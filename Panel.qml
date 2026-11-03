import QtQuick
import Quickshell
import Quickshell.Io
import qs.Commons
import qs.Ui
import "components"

// NOTE: Composition root. State, derived values and all `peace` IO live in
// NOTE: PeaceModel; view pieces live in components/ + components/ui.
Panel {
  id: root
  moduleName: "io.github.sohanemon.omapeace"
  ipcTarget: "io.github.sohanemon.omapeace"
  manageIpc: false

  property var hostWidget: null
  property var anchorItem: null
  readonly property var barIdentity: hostWidget || root
  readonly property string peaceExecutable: decodeURIComponent(String(Qt.resolvedUrl("bin/peace")).replace(/^file:\/\//, ""))
  property var settings: hostWidget && hostWidget.settings ? hostWidget.settings : null

  PeaceModel {
    id: model

    hostWidget: root.hostWidget
    bar: root.bar
    moduleName: root.moduleName
    peaceExecutable: root.peaceExecutable
    settings: root.settings
    active: root.opened
  }

  function open() {
    root.controller.show()
    model.open()
  }

  function close() {
    root.closePickers()
    root.controller.hide()
  }

  function toggle() {
    if (root.opened)
      root.close()
    else
      root.open()
  }

  function closeForPopoutSwitch() {
    root.close()
  }

  function openTranslationPicker() {
    model.openTranslationPicker()
  }

  function openWBWPicker() {
    model.openWBWPicker()
  }

  function closePickers() {
    model.closePickers()
    keyCatcher.forceActiveFocus()
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
      blocked: recitationControls.blocksKeys || model.translationPickerOpen || model.wbwPickerOpen
      onCloseRequested: {
        if (model.translationPickerOpen || model.wbwPickerOpen) {
          root.closePickers()
        } else {
          root.close()
        }
      }

      Column {
        id: content

        width: parent.width
        spacing: Style.space(12)

        PanelHeader {
          width: parent.width
          foreground: model.foreground
          fontFamily: model.fontFamily
          title: "Peace"
          playbackStatus: model.playbackStatus
          statusActive: model.isPlaying || model.isPreparing
        }

        PanelSeparator {
          foreground: model.foreground
        }

        Column {
          width: parent.width
          spacing: Style.space(10)

          PlaybackCard {
            width: parent.width
            height: Style.space(168)
            foreground: model.foreground
            fontFamily: model.fontFamily
            artworkPath: model.artworkPath
            fallbackArtwork: Qt.resolvedUrl("assets/backgrounds/01-pattern.png")
            isPlaying: model.isPlaying
            isPaused: model.isPaused
            isPreparing: model.isPreparing
            busy: model.pendingListenArgs !== null
            onPrevRequested: model.prev()
            onNextRequested: model.next()
            onPlayPauseRequested: model.playOrPause()
            onStopRequested: model.stop()
            onWallpaperNextRequested: model.nextWallpaper()
            onArtworkLoadFailed: model.artworkPath = ""
          }

          TrackInfoRow {
            width: parent.width
            foreground: model.foreground
            fontFamily: model.fontFamily
            surahName: model.currentSurahName
            reciterName: model.reciterName
            ayahNumber: model.currentAyahNumber
          }

          RecitationControls {
            id: recitationControls

            width: parent.width
            foreground: model.foreground
            fontFamily: model.fontFamily
            reciterId: model.reciterID
            reciterOptions: model.reciterOptions
            followEnabled: model.followEnabled
            basmalaEnabled: model.basmalaEnabled
            randomBackground: model.randomBackground
            showTranslation: model.showTranslation
            showWordTranslation: model.showWordTranslation
            showReference: model.showReference
            useThemeColors: model.useThemeColors
            selectedTranslationName: model.selectedTranslationName
            selectedWBWLanguageName: model.selectedWBWLanguageName
            referenceText: model.referenceText
            onReferenceEdited: function(text) {
              if (model.referenceText !== text)
                model.referenceText = text
            }
            onReciterSelected: function(value) {
              model.reciterID = value
            }
            onFollowToggled: model.followEnabled = !model.followEnabled
            onBasmalaToggled: model.basmalaEnabled = !model.basmalaEnabled
            onRandomToggled: model.randomBackground = !model.randomBackground
            onShowTranslationToggled: model.showTranslation = !model.showTranslation
            onShowWordTranslationToggled: model.showWordTranslation = !model.showWordTranslation
            onShowReferenceToggled: model.showReference = !model.showReference
            onUseThemeColorsToggled: model.useThemeColors = !model.useThemeColors
            onTranslationGearRequested: root.openTranslationPicker()
            onWbwGearRequested: root.openWBWPicker()
          }
        }
      }

      PickerOverlay {
        id: pickerOverlay

        anchors.fill: parent
        z: 100
        visible: model.translationPickerOpen || model.wbwPickerOpen
        foreground: model.foreground
        fontFamily: model.fontFamily
        translationOpen: model.translationPickerOpen
        wbwOpen: model.wbwPickerOpen
        translations: model.filteredTranslations
        wbwLanguages: model.filteredWBWLanguages
        selectedTranslation: model.selectedTranslation
        selectedWBWLanguage: model.selectedWBWLanguage
        translationsError: model.translationsError
        wbwError: model.wbwLanguagesError
        translationCacheCount: model.allTranslations.length
        wbwCacheCount: model.allWBWLanguages.length
        translationsLoading: model.translationsLoading
        wbwLoading: model.wbwLanguagesLoading
        onCloseRequested: root.closePickers()
        onTranslationSelected: function(value) {
          model.persistSettings(value, null)
          root.closePickers()
        }
        onWbwSelected: function(value) {
          model.persistSettings(null, value)
          root.closePickers()
        }
        onTranslationSearchEdited: function(text) {
          model.translationSearchQuery = text
        }
        onWbwSearchEdited: function(text) {
          model.wbwSearchQuery = text
        }
      }
    }
  }
}
