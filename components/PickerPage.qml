import QtQuick
import qs.Commons
import qs.Ui
import "ui"

// Page hosting the two search pickers, shown in place of the settings page.
Item {
  id: page

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

  // INFO: fixed page height so the panel can size itself while a list scrolls
  implicitHeight: Style.space(440)

  width: parent.width

  signal closeRequested()
  signal translationSelected(string value)
  signal wbwSelected(string value)
  signal translationSearchEdited(string text)
  signal wbwSearchEdited(string text)

  SearchablePicker {
    id: translationPicker

    visible: page.translationOpen
    open: page.translationOpen
    anchors.fill: parent
    title: "Select Translation"
    countSuffix: "available"
    placeholder: "Search name, author, or language..."
    closeName: "Back to settings"
    items: page.translations
    selectedValue: page.selectedTranslation
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
    error: page.translationsError
    hasCachedItems: page.translationCacheCount > 0
    loading: page.translationsLoading && page.translationCacheCount === 0
    loadingLabel: "Loading translations…"
    emptyText: "No translations found matching \"" + translationPicker.searchQuery + "\""
    foreground: page.foreground
    fontFamily: page.fontFamily

    onSelected: function(value) {
      page.translationSelected(value)
    }
    onCloseRequested: page.closeRequested()
    onSearchEdited: function(text) {
      page.translationSearchEdited(text)
    }
  }

  SearchablePicker {
    id: wbwPicker

    visible: page.wbwOpen
    open: page.wbwOpen
    anchors.fill: parent
    title: "Word-by-Word Language"
    countSuffix: "verified languages"
    placeholder: "Search language name or code..."
    closeName: "Back to settings"
    items: page.wbwLanguages
    selectedValue: page.selectedWBWLanguage
    valueOf: function(modelData) { return String(modelData.iso_code) }
    titleOf: function(modelData) {
      return String(modelData.name || modelData.iso_code)
    }
    secondaryOf: function(modelData) {
      var nativeName = (modelData.native_name && modelData.native_name !== modelData.name ? modelData.native_name + " • " : "")
      return nativeName + "ISO: " + modelData.iso_code
    }
    tagOf: function(modelData) { return String(modelData.iso_code || "").toUpperCase() }
    rowHeight: Style.space(48)
    error: page.wbwError
    hasCachedItems: page.wbwCacheCount > 0
    loading: page.wbwLoading && page.wbwCacheCount === 0
    loadingLabel: "Loading languages…"
    emptyText: "No languages found matching \"" + wbwPicker.searchQuery + "\""
    foreground: page.foreground
    fontFamily: page.fontFamily

    onSelected: function(value) {
      page.wbwSelected(value)
    }
    onCloseRequested: page.closeRequested()
    onSearchEdited: function(text) {
      page.wbwSearchEdited(text)
    }
  }
}
