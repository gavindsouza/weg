{
  description = "weg development shell";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs = { nixpkgs, ... }:
    let
      forAllSystems = f: nixpkgs.lib.genAttrs [ "x86_64-linux" "aarch64-linux" "x86_64-darwin" "aarch64-darwin" ]
        (system: f nixpkgs.legacyPackages.${system});
    in
    {
      devShells = forAllSystems (pkgs: {
        default = pkgs.mkShell {
          packages = [ pkgs.go_1_27 ];
          # Use the pinned toolchain; never let go.mod trigger a toolchain download.
          GOTOOLCHAIN = "local";
        };
      });
    };
}
