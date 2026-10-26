# Release key

`tabletdeck-release.jks` is a sideload signing key for the Android app, made for
Tablet Deck; it isn't registered with Google Play. Its passwords are in
`keystore.properties`, which `app/build.gradle.kts` reads to sign the release APK.

Both files are gitignored. **Back them up somewhere safe.** Android only installs an
update over an existing app when it's signed with the same key, so if the key is lost,
players have to uninstall the app (which clears its pairing) before installing a build
signed with a new one.

To make a new key (only for a fresh start):

```sh
keytool -genkeypair -keystore tabletdeck-release.jks -storetype PKCS12 -alias tabletdeck \
  -keyalg RSA -keysize 4096 -validity 10000 -dname "CN=Tablet Deck, O=Xattribution"
```

then write `keystore.properties`:

```properties
storeFile=keystore/tabletdeck-release.jks
storePassword=...
keyAlias=tabletdeck
keyPassword=...
```

Without `keystore.properties` the release build still builds, unsigned.
