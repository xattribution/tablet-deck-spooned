package app

import "testing"

// Lines in the shape the game writes them (names and ids made up).
func TestBattleFromGameLog(t *testing.T) {
	a, _ := startApp(t, "Pilot")
	for _, l := range []string{
		`<2026-05-22T10:20:00.000Z> [Notice] <Legacy login response> [CIG-net] User Login Success - Handle[TestPilot] - Time[1] nickname="TestPilot"` + "\n",
		`<2026-05-22T10:20:01.000Z> [Notice] <Vehicle Control Flow> CVehicleMovementBase::SetDriver: Local client node [111] requesting control token for 'ORIG_m80_200000777639' [200000777639] [Team_CGP4][Vehicle]` + "\n",
		`<2026-05-22T10:25:10.983Z> [Notice] <Debug Hostility Events> [OnHandleHit] Hit FROM PU_Pilots-Human-Pirate-Pilot_Light_200000780096 TO ORIG_m80_200000777639. Being sent to child TestPilot [Team_MissionFeatures][HitInfo]` + "\n",
		`<2026-05-22T10:25:11.983Z> [Notice] <Debug Hostility Events> [OnHandleHit] Hit FROM TestPilot TO DRAK_Buccaneer_200000779834. Being sent to child PU_Pilot_1 [Team_MissionFeatures][HitInfo]` + "\n",
	} {
		a.gameLogLine(l)
	}
	st := a.battleNow()
	if !st.Fire || st.From != "Pirate" || st.Ship != "ORIG m80" || st.Hits != 1 {
		t.Fatalf("after a hit: %+v", st)
	}
	a.gameLogLine(`<x> [Notice] <Vehicle Destruction> CVehicle::OnAdvanceDestroyLevel: Vehicle 'ORIG_m80_200000777639' [200000777639] in zone 'z' [pos x: 1] driven by 'TestPilot' [111] advanced from destroy level 0 to 1 caused by 'PU_Pilots-Human-Pirate-Pilot_Light_200000780096' [2] with 'Combat' [Team_CGP4][Vehicle]` + "\n")
	if a.battleNow().Level != 1 {
		t.Fatalf("ship not disabled: %+v", a.battleNow())
	}
	a.gameLogLine(`<x> [Notice] <Actor Death> CActor::Kill: 'PU_Pilot_9' [9] in zone 'z' killed by 'TestPilot' [111] using 'gun' [Class unknown] with damage type 'Bullet' from direction x: 0 [Team_ActorTech][Actor]` + "\n")
	a.gameLogLine(`<x> [Notice] <Actor Death> CActor::Kill: 'TestPilot' [111] in zone 'z' killed by 'PU_Pilot_9' [9] using 'gun' [Class unknown] with damage type 'Bullet' from direction x: 0 [Team_ActorTech][Actor]` + "\n")
	if st := a.battleNow(); !st.Dead || st.Kills != 1 || st.Fire {
		t.Fatalf("after dying: %+v", st)
	}
}
