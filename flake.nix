{
  description = "triton - a terminal UI for live-coding music with Tidal Cycles";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
  };

  outputs =
    { self, nixpkgs }:
    let
      systems = [
        "x86_64-linux"
        "aarch64-linux"
        "aarch64-darwin"
      ];
      forAllSystems = f: nixpkgs.lib.genAttrs systems (system: f system nixpkgs.legacyPackages.${system});

      # SuperCollider with sc3-plugins merged in, so SuperDirt's synth
      # sound banks (superpiano, supergong, ...) are available. Empty on
      # platforms where nixpkgs has no SuperCollider (Apple Silicon macOS);
      # users there install SuperCollider themselves.
      # supercollider-with-sc3-plugins already contains SuperCollider and
      # wraps sclang/scsynth with the sc3-plugins class and UGen paths.
      # nixpkgs builds scsynth with JACK as its only audio backend. On
      # systems where PipeWire serves audio (no jackd running), scsynth
      # cannot open a device, exits, and crashes in UIUGens' atexit
      # handler. The wrappers installed below make the tools fall back to
      # PipeWire's JACK emulation library in that case. sclang must be
      # wrapped too: its wrapper re-prefixes PATH with the unwrapped
      # store bin, so the scsynth it spawns only gets the fallback
      # through the inherited environment.
      supercolliderPackages =
        pkgs:
        if pkgs.lib.meta.availableOn pkgs.stdenv.hostPlatform pkgs.supercollider-with-sc3-plugins then
          [
            (pkgs.symlinkJoin {
              name = "supercollider-superdirt";
              paths = [ pkgs.supercollider-with-sc3-plugins ];
              nativeBuildInputs = pkgs.lib.optionals pkgs.stdenv.hostPlatform.isLinux [ pkgs.makeWrapper ];
              postBuild = pkgs.lib.optionalString pkgs.stdenv.hostPlatform.isLinux ''
                for prog in sclang scsynth supernova; do
                  [ -e "$out/bin/$prog" ] || continue
                  wrapProgram "$out/bin/$prog" --run '
                    rt="''${XDG_RUNTIME_DIR:-/run/user/$(id -u)}"
                    if [ -S "$rt/pipewire-0" ] && [ ! -S "$rt/jack_0" ]; then
                      export LD_LIBRARY_PATH=@pwJackLib@''${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}
                    fi'
                  substituteInPlace "$out/bin/$prog" \
                    --replace-warn @pwJackLib@ ${pkgs.lib.getOutput "jack" pkgs.pipewire}/lib
                done
              '';
            })
          ]
        else
          [ ];

      mkPackages =
        system: pkgs:
        let
          sc = supercolliderPackages pkgs;

          # GHC that sees Tidal Cycles, so plain `ghci` can boot Tidal without
          # `cabal install --lib tidal --package-env .`.
          ghc-with-tidal = pkgs.haskellPackages.ghcWithPackages (hp: [ hp.tidal ]);
        in
        {
          default = self.packages.${system}.triton;

          inherit ghc-with-tidal;

          triton = pkgs.buildGoModule {
            pname = "triton";
            version = "0.1.0";
            src = ./.;
            vendorHash = "sha256-5KM/LJaYhu/7JAYLuGUgRYiL3evX0yryPSuXAh/Urro=";
            # Stripping symbols and DWARF shrinks the binary by about a third;
            # Go panics still name functions without them.
            ldflags = [
              "-s"
              "-w"
            ];
            nativeBuildInputs = [ pkgs.makeWrapper ];
            postInstall = ''
                install -Dm644 start-superdirt.scd install-superdirt.scd -t $out/share/triton
                cat > $out/bin/triton-install-superdirt <<'SCRIPT'
              #!@shell@
              exec sclang @out@/share/triton/install-superdirt.scd "$@"
              SCRIPT
                substituteInPlace $out/bin/triton-install-superdirt \
                  --replace-warn @shell@ ${pkgs.runtimeShell} \
                  --replace-warn @out@ $out
                chmod +x $out/bin/triton-install-superdirt
            '';
            postFixup = ''
              wrapProgram $out/bin/triton \
                --prefix PATH : ${pkgs.lib.makeBinPath ([ ghc-with-tidal ] ++ sc)} \
                --set-default GHC_ENVIRONMENT -
              wrapProgram $out/bin/triton-install-superdirt \
                --prefix PATH : ${pkgs.lib.makeBinPath sc}
            '';
            meta.mainProgram = "triton";
            meta.description = "Terminal UI for live-coding music with Tidal Cycles";
            meta.license = pkgs.lib.licenses.mit;
            meta.platforms = systems;
          };
        }
        // nixpkgs.lib.optionalAttrs (sc != [ ]) {
          supercollider-superdirt = builtins.head sc;
        };

      # `nix flake check` builds the triton package and smoke-tests the
      # Tidal boot path end to end: the bundled boot script must import
      # Sound.Tidal.Boot and bind the tidal aliases under the GHC that
      # ships with triton, even when a stale cabal package environment
      # file sits in the working directory (GHC_ENVIRONMENT=- ignores it).
      # The stale file is named after the GHC build platform and version
      # that GHC itself reports, so no filename is hardcoded.
      mkChecks =
        system: pkgs:
        let
          ghc-with-tidal = self.packages.${system}.ghc-with-tidal;
        in
        {
          triton = pkgs.runCommand "triton-tidal-smoke-test" { } ''
            triton=${self.packages.${system}.triton}/bin/triton
            if "$triton" --no-such-flag >usage.log 2>&1; then
              echo "triton accepted an unknown flag" >&2
              exit 1
            fi
            grep -q "usage: triton" usage.log

            ghc=${ghc-with-tidal}/bin/ghc
            mkdir cwd && cd cwd
            plat=$($ghc --print-build-platform)
            printf 'clear-package-db\nglobal-package-db\npackage-id base-0\n' \
              > ".ghc.environment.''${plat%%-*}-''${plat##*-}-$($ghc --numeric-version)"
            # The boot script is UTF-8 (em dashes in comments); the sandbox
            # has no locale, so GHCi would decode it as ASCII and silently
            # skip it. Force a UTF-8 locale before running GHCi.
            LC_ALL=C.UTF-8 GHC_ENVIRONMENT=- ${ghc-with-tidal}/bin/ghci -v0 \
              -ghci-script ${./boot/BootTidal-triton.hs} \
              -e ':t d1' -e 'putStrLn "boot-smoke-ok"' > boot.log 2>&1
            grep -q 'd1 ::' boot.log || { cat boot.log; exit 1; }
            grep -q 'boot-smoke-ok' boot.log || { cat boot.log; exit 1; }
            touch $out
          '';
        };
    in
    {
      packages = forAllSystems mkPackages;

      checks = forAllSystems mkChecks;

      formatter = forAllSystems (system: pkgs: pkgs.nixfmt);

      devShells = forAllSystems (
        system: pkgs: {
          default = pkgs.mkShell {
            packages = [
              pkgs.go
              self.packages.${system}.ghc-with-tidal
              pkgs.cabal-install
            ]
            ++ supercolliderPackages pkgs;
            # Ignore stale cabal .ghc.environment files, same as the triton
            # wrapper does; ghc-with-tidal already sees Tidal.
            GHC_ENVIRONMENT = "-";
          };
        }
      );
    };
}
