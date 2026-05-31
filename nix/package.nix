# AIDEV-NOTE: buildGoModule with vendorHash = null uses the checked-in vendor/
# directory, so the build is offline and reproducible. The binary name comes
# from the cmd/statshed package directory. Takes the full pkgs set so that
# pkgs.buildGoModule arrives pre-applied with the Go toolchain.
{ pkgs ? import <nixpkgs> { } }:

let
  inherit (pkgs) lib;
in
pkgs.buildGoModule {
  pname = "statshed-cli";
  version = "1.0.2";

  src = lib.cleanSource ../.;

  vendorHash = null; # dependencies are vendored under ./vendor

  subPackages = [ "cmd/statshed" ];

  ldflags = [ "-s" "-w" "-X" "main.version=1.0.2" ];

  nativeBuildInputs = [ pkgs.installShellFiles ];

  postInstall = ''
    installManPage docs/statshed.1
    installShellCompletion --cmd statshed \
      --bash <($out/bin/statshed completion bash) \
      --zsh  <($out/bin/statshed completion zsh) \
      --fish <($out/bin/statshed completion fish)
  '';

  meta = {
    description = "Command-line interface for the StatShed status dashboard";
    homepage = "https://github.com/statshed/statshed-cli";
    license = lib.licenses.cc0;
    mainProgram = "statshed";
    platforms = lib.platforms.unix;
  };
}
