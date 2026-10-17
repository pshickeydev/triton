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
            vendorHash = "sha256-khfl2gO7OcaDyJpskIn0Og5VP5K6tC/DXz8uCD+pD68=";
            # Stripping symbols and DWARF shrinks the binary by about a third;
            # Go panics still name functions without them.
            ldflags = [ "-s" "-w" ];
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
            passthru.tests.tidal-import = pkgs.runCommand "triton-tidal-import-test" { } ''
              # ghci must boot Tidal even with a stale cabal .ghc.environment
              # file in the working directory; GHC_ENVIRONMENT=- ignores it.
              mkdir cwd && cd cwd
              printf 'clear-package-db\nglobal-package-db\npackage-id base-4.20.2.0-inplace\n' \
                > .ghc.environment.x86_64-linux-9.10.3
              GHC_ENVIRONMENT=- ${ghc-with-tidal}/bin/ghci \
                -e 'putStrLn "tidal-ok"' > out.log 2>&1
              grep -q tidal-ok out.log || (cat out.log; exit 1)
              touch $out
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
    in
    {
      packages = forAllSystems mkPackages;

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
