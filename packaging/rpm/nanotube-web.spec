Name:           nanotube-web
Version:        0.1.0
Release:        1%{?dist}
Summary:        Ultra-light desktop YouTube client for Linux

License:        MIT
URL:            https://github.com/nanotube/nanotube-web
Source0:        %{name}-%{version}.tar.gz

BuildRequires:  golang >= 1.22
BuildRequires:  gcc
BuildRequires:  pkgconfig
BuildRequires:  pkgconfig(gtk4)
BuildRequires:  pkgconfig(webkitgtk-6.0)
BuildRequires:  nodejs
BuildRequires:  npm

Requires:       glibc >= 2.38
Requires:       gtk4
Requires:       webkitgtk6.0
Requires:       ca-certificates
Recommends:     yt-dlp

%description
NanoTube Web is a fast, battery-efficient, privacy-friendly YouTube
desktop client built with Go, Svelte, and Tailwind CSS. It features local
MMR recommendations, custom HTML5/HLS web player with WebAudio DSP, and TV Mode.

%prep
%setup -q

%build
cd frontend
npm run build:nanotube
cd ..
go build -buildvcs=false -trimpath \
    -ldflags="-s -w -X github.com/nanotube/nanotube-web/internal/diagnostics.Version=%{version}" \
    -o bin/nanotube-web ./cmd/nanotube-web

%install
rm -rf $RPM_BUILD_ROOT
install -d $RPM_BUILD_ROOT%{_bindir}
install -d $RPM_BUILD_ROOT%{_datadir}/nanotube-web/frontend/dist/nanotube
install -d $RPM_BUILD_ROOT%{_datadir}/applications
install -d $RPM_BUILD_ROOT%{_datadir}/icons/hicolor/scalable/apps

install -m 755 bin/nanotube-web $RPM_BUILD_ROOT%{_bindir}/nanotube-web
cp -a frontend/dist/nanotube/. $RPM_BUILD_ROOT%{_datadir}/nanotube-web/frontend/dist/nanotube/
install -m 644 packaging/nanotube-web.desktop $RPM_BUILD_ROOT%{_datadir}/applications/nanotube-web.desktop
install -m 644 packaging/nanotube-web.svg $RPM_BUILD_ROOT%{_datadir}/icons/hicolor/scalable/apps/nanotube-web.svg

%files
%{_bindir}/nanotube-web
%{_datadir}/nanotube-web/
%{_datadir}/applications/nanotube-web.desktop
%{_datadir}/icons/hicolor/scalable/apps/nanotube-web.svg

%changelog
* Mon Aug 31 2026 NanoTube Team <maintainer@nanotube.local> - 0.1.0-1
- Align build and installed assets with the NanoTube-only preview
