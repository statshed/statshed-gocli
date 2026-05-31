# Non-flake entry point: `nix-build` or `nix-shell`.
{ pkgs ? import <nixpkgs> { } }:

import ./nix/package.nix { inherit pkgs; }
