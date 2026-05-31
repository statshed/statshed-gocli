# AIDEV-NOTE: Built from a tarball that includes vendor/, so the build runs
# fully offline with -mod=vendor. Pass --define "version X.Y.Z" or rely on the
# default below.
%global goipath github.com/statshed/statshed-cli
%{!?version: %global version 1.0.2}

# Skip debuginfo: the binary is built with -s -w (stripped).
%global debug_package %{nil}

Name:           statshed-cli
Version:        %{version}
Release:        1%{?dist}
Summary:        Command-line interface for StatShed status dashboard

License:        CC0-1.0
URL:            https://github.com/statshed/statshed-cli
Source0:        %{name}-%{version}.tar.gz

BuildRequires:  golang >= 1.21
BuildRequires:  make

%description
StatShed CLI provides the "statshed" command for interacting with the StatShed
status dashboard: submitting job status, streaming and wrapping command output
as progress updates, checking health, listing groups and jobs, and managing
timeout configuration. It ships as a single dependency-free Go binary.

%prep
%autosetup -n %{name}-%{version}

%build
export CGO_ENABLED=0
export GOFLAGS=-mod=vendor
export GOTOOLCHAIN=local
export GOCACHE=%{_builddir}/.gocache
go build -trimpath -ldflags "-s -w -X main.version=%{version}" \
    -o statshed ./cmd/statshed

%check
export GOFLAGS=-mod=vendor
export GOTOOLCHAIN=local
export GOCACHE=%{_builddir}/.gocache
go test ./...

%install
install -D -m0755 statshed %{buildroot}%{_bindir}/statshed
install -D -m0644 docs/statshed.1 %{buildroot}%{_mandir}/man1/statshed.1
install -d %{buildroot}%{_datadir}/bash-completion/completions
./statshed completion bash > %{buildroot}%{_datadir}/bash-completion/completions/statshed
install -d %{buildroot}%{_datadir}/zsh/site-functions
./statshed completion zsh > %{buildroot}%{_datadir}/zsh/site-functions/_statshed
install -d %{buildroot}%{_datadir}/fish/vendor_completions.d
./statshed completion fish > %{buildroot}%{_datadir}/fish/vendor_completions.d/statshed.fish

%files
%license LICENSE
%doc README.md CHANGELOG.md
%{_bindir}/statshed
%{_mandir}/man1/statshed.1*
%{_datadir}/bash-completion/completions/statshed
%{_datadir}/zsh/site-functions/_statshed
%{_datadir}/fish/vendor_completions.d/statshed.fish

%changelog
* Sun May 31 2026 Sean <jafo00+oss@gmail.com> - 1.0.2-1
- Initial Go release.
