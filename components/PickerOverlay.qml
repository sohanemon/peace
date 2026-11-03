import QtQuick
import qs.Commons
import qs.Ui
import "ui"

// Overlay hosting the two search pickers over the whole panel.
Item {
  id: overlay

  property bool translationOpen: false
  property bool wbwOpen: false
  property var translations: []
  property var wbwLanguages: []
  property string selectedTranslation: ""
  property string selectedWBWLanguage: ""
  property string translationsError: ""
  property string wbwError: ""
  property int translationCacheCount: 0
  property int wbwCacheCount: 0
  property bool translationsLoading: false
  property bool wbwLoading: false
  property color foreground: Color.foreground
  property string fontFamily: "monospace"

  // INFO: re-exported so the panel can filter against live search text
  readonly property alias translationSearchQuery: translationPickerCard.searchQuery
  readonly property alias wbwSearchQuery: wbwPickerCard.searchQuery

  signal closeRequested()
  signal translationSelected(string value)
  signal wbwSelected(string value)
  signal translationSearchEdited(string text)
  signal wbwSearchEdited(string text)

  Rectangle {
    anchors.fill: parent
    color: Qt.rgba(0, 0, 0, 0.72)
    radius: Style.cornerRadius

    MouseArea {
      anchors.fill: parent
      onClicked: overlay.closeRequested()
    }
  }

  SearchablePicker {
    id: translationPickerCard

    visible: overlay.translationOpen
    open: overlay.translationOpen
    anchors.fill: parent
    anchors.margins: Style.space(12)
    title: "Select Translation"
    countSuffix: "available"
    placeholder: "Search name, author, or language..."
    closeName: "Close translation picker"
    items: overlay.translations
    selectedValue: overlay.selectedTranslation
    valueOf: function(modelData) { return String(modelData.id) }
    titleOf: function(modelData) {
      return String(modelData.name || "Translation " + modelData.id)
    }
    secondaryOf: function(modelData) {
      var author = (modelData.author && modelData.author !== modelData.name ? modelData.author + " • " : "")
      var language = modelData.language ? (modelData.language.charAt(0).toUpperCase() + modelData.language.slice(1)) : ""
      return author + language
    }
    tagOf: function(modelData) { return String(modelData.language || "").toUpperCase() }
    rowHeight: Style.space(52)
    error: overlay.translationsError
    hasCachedItems: overlay.translationCacheCount > 0
    loading: overlay.translationsLoading && overlay.translationCacheCount === 0
    loadingLabel: "Loading translations…"
    emptyText: "No translations found matching \"" + overlay.translationSearchQuery + "\""
    foreground: overlay.foreground
    fontFamily: overlay.fontFamily

    onSelected: function(value) {
      overlay.translationSelected(value)
    }
    onCloseRequested: overlay.closeRequested()
    onSearchEdited: function(text) {
      overlay.translationSearchEdited(text)
    }
  }

  SearchablePicker {
    id: wbwPickerCard

    visible: overlay.wbwOpen
    open: overlay.wbwOpen
    anchors.fill: parent
    anchors.margins: Style.space(12)
    title: "Word-by-Word Language"
    countSuffix: "verified languages"
    placeholder: "Search language name or code..."
    closeName: "Close WBW language picker"
    items: overlay.wbwLanguages
    selectedValue: overlay.selectedWBWLanguage
    valueOf: function(modelData) { return String(modelData.iso_code) }
    titleOf: function(modelData) {
      return String(modelData.name || modelData.iso_code)
    }
    secondaryOf: function(modelData) {
      var native = (modelData.native_name && modelData.native_name !== modelData.name ? modelData.native_name + " • " : "")
      return native + "ISO: " + modelData.iso_code
    }
    tagOf: function(modelData) { return String(modelData.iso_code || "").toUpperCase() }
    rowHeight: Style.space(48)
    error: overlay.wbwError
    hasCachedItems: overlay.wbwCacheCount > 0
    loading: overlay.wbwLoading && overlay.wbwCacheCount === 0
    loadingLabel: "Loading languages…"
    emptyText: "No languages found matching \"" + overlay.wbwSearchQuery + "\""
    foreground: overlay.foreground
    fontFamily: overlay.fontFamily

    onSelected: function(value) {
      overlay.wbwSelected(value)
    }
    onCloseRequested: overlay.closeRequested()
    onSearchEdited: function(text) {
      overlay.wbwSearchEdited(text)
    }
  }
}
