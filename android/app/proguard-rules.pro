# The deck page calls these by name through window.TabletDeckNative.
-keepattributes JavascriptInterface
-keep class dev.xattribution.tabletdeck.Bridge { *; }
-keepclassmembers class dev.xattribution.tabletdeck.Bridge {
    @android.webkit.JavascriptInterface <methods>;
}
