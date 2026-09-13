Quarks.install("SuperDirt");
// Wait for install to complete before exiting
Routine {
    5.wait;
    "SuperDirt install attempted. Recompile to load.".postln;
    0.exit;
}.play;
