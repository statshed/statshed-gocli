{
  description = "StatShed CLI - command-line tool for the StatShed status dashboard";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs = { self, nixpkgs }:
    let
      systems = [ "x86_64-linux" "aarch64-linux" "x86_64-darwin" "aarch64-darwin" ];
      forAll = f: nixpkgs.lib.genAttrs systems (system: f system (import nixpkgs { inherit system; }));
    in
    {
      packages = forAll (system: pkgs: rec {
        statshed-cli = import ./nix/package.nix { inherit pkgs; };
        default = statshed-cli;
      });

      apps = forAll (system: pkgs: rec {
        statshed = {
          type = "app";
          program = "${self.packages.${system}.statshed-cli}/bin/statshed";
        };
        default = statshed;
      });

      devShells = forAll (system: pkgs: {
        default = pkgs.mkShell {
          packages = [ pkgs.go pkgs.gopls pkgs.gotools ];
        };
      });

      # Overlay so other flakes can add statshed-cli to their nixpkgs.
      overlays.default = final: _prev: {
        statshed-cli = final.callPackage ./nix/package.nix { };
      };
    };
}
