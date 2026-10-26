package dev.xattribution.tabletdeck

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test

class BeaconTest {
    private fun parse(json: String, ip: String = "192.168.1.20") =
        json.toByteArray().let { Beacon.parse(it, it.size, ip, 1000L) }

    @Test
    fun parsesAValidBeacon() {
        val h = parse("""{"app":"tabletdeck","id":"a1b2","name":"Gaming PC","port":47615,"ver":"2.0.0"}""")!!
        assertEquals(Host("a1b2", "Gaming PC", "192.168.1.20", 47615, "2.0.0", 1000L), h)
        assertEquals("192.168.1.20:47615", h.address)
    }

    @Test
    fun ignoresOtherAppsAndBadFields() {
        assertNull(parse("""{"app":"other","id":"a","name":"x","port":47615}"""))
        assertNull(parse("""{"app":"tabletdeck","id":"","name":"x","port":47615}"""))
        assertNull(parse("""{"app":"tabletdeck","id":"a","name":"x","port":0}"""))
        assertNull(parse("""{"app":"tabletdeck","id":"a","name":"x","port":70000}"""))
        assertNull(parse("""{"app":"tabletdeck","id":"a","name":"x","port":"47615"}"""))
        assertNull(parse("""{"app":"tabletdeck","id":"a","name":"x"}"""))
        assertNull(parse("not json"))
        assertNull(parse(""))
    }

    @Test
    fun trimsLongNamesAndFallsBackToTheAddress() {
        val long = "x".repeat(80)
        assertEquals(40, parse("""{"app":"tabletdeck","id":"a","name":"$long","port":1}""")!!.name.length)
        assertEquals("10.0.0.5", parse("""{"app":"tabletdeck","id":"a","name":"  ","port":1}""", "10.0.0.5")!!.name)
    }

    @Test
    fun readsOnlyTheReceivedLength() {
        val bytes = """{"app":"tabletdeck","id":"a","name":"PC","port":47615}GARBAGE""".toByteArray()
        assertEquals("PC", Beacon.parse(bytes, bytes.size - 7, "192.168.1.20", 0L)!!.name)
    }

    @Test
    fun hostListJsonRoundTrips() {
        val json = Beacon.toJson(listOf(Host("a", "PC \"one\"", "192.168.1.20", 47615, "2.0.0", 5L)))
        val o = org.json.JSONArray(json).getJSONObject(0)
        assertEquals("PC \"one\"", o.getString("name"))
        assertEquals(47615, o.getInt("port"))
        assertEquals(5L, o.getLong("lastSeen"))
    }
}
