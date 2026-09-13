-- BootTidal-triton.hs — a copy of the upstream BootTidal.hs that adds a
-- second OSC target for the triton pattern tracing cursor.
--
-- Requires Tidal >= 1.9 (the Sound.Tidal.Boot / mkTidalWith API).
-- GHCi runs scripts line by line, so every statement stays on one line.

:set -fno-warn-orphans -Wno-type-defaults -XMultiParamTypeClasses -XOverloadedStrings
:set prompt ""

-- Import all the boot functions and aliases.
import Sound.Tidal.Boot

default (Rational, Integer, Double, Pattern String)

-- Second OSC target: the triton TUI listens on this port and uses
-- the source positions in each /editor/highlights message to highlight
-- the mini-notation fragment that is currently sounding.
let editorTarget = Target {oName = "charm-cursor", oAddress = "127.0.0.1", oPort = 6013, oBusPort = Nothing, oLatency = 0.05, oWindow = Nothing, oSchedule = Pre BundleStamp, oHandshake = False}

let charmOscMap = [(superdirtTarget {oLatency = 0.05, oAddress = "127.0.0.1", oPort = 57120}, [superdirtShape]), (editorTarget, [OSCContext "/editor/highlights"])]

-- A lowered cFrameTimespan keeps highlight refreshes responsive.
tidalInst <- mkTidalWith charmOscMap (defaultConfig {cFrameTimespan = 1/50, cProcessAhead = 1/20})

-- This orphan instance makes the boot aliases work!
-- It has to go after you define 'tidalInst'.
instance Tidally where tidal = tidalInst

:set prompt "tidal> "
:set prompt-cont ""
