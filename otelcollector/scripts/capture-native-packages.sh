#!/bin/bash
set -euo pipefail

root=/native-rootfs
mkdir -p /opt/native-packages "$root/var/lib/rpmmanifest"
files=/opt/native-packages/files.tsv
manifest=/opt/native-packages/rpmmanifest.tsv
: > "$files"
: > "$manifest"
format='%{NAME}\t%{VERSION}-%{RELEASE}\t%{INSTALLTIME}\t%{BUILDTIME}\t%{VENDOR}\t(none)\t%{SIZE}\t%{ARCH}\t%{EPOCHNUM}\t%{SOURCERPM}\n'

capture() {
    local consumer=$1 path owner version hash
    shift
    for path in "$@"; do
        test -f "$path"
        owner=$(rpm -qf --qf '%{NAME}' "$path")
        version=$(rpm -q "$owner" --qf '%{EPOCHNUM}:%{VERSION}-%{RELEASE}')
        version=${version#0:}
        if [[ "$path" == /opt/* ]]; then
            hash=$(sha256sum "$path")
        else
            mkdir -p "$root${path%/*}"
            cp -L "$path" "$root$path"
            hash=$(sha256sum "$root$path")
        fi
        printf '%s\t%s\t%s\t%s\t%s\n' "$consumer" "$owner" "$version" "$path" "${hash%% *}" >> "$files"
        rpm -q "$owner" --qf "$format" >> "$manifest"
    done
}

# ponytail: stage the native COPY set once; the same bytes supply hashes and the final image.
capture metricsext2 /usr/sbin/MetricsExtension \
    /usr/lib/libboost_filesystem.so.* /usr/lib/libcpprest.so.* /usr/lib/libuuid.so.1 \
    /usr/lib/libstdc++.so.6 /usr/lib/libm.so.6 /usr/lib/libgcc_s.so.1 /usr/lib/libc.so.6 \
    /usr/lib/libboost_atomic.so.* /usr/lib/libbrotlidec.so.1 /usr/lib/libbrotlienc.so.1 \
    /usr/lib/libz.so.1 /usr/lib/libbrotlicommon.so.1
capture azure-mdsd /usr/sbin/mdsd \
    /usr/lib/libpthread.so.0 /usr/lib/libdl.so.2 /usr/lib/libsymcrypt.so.103 \
    /usr/lib/librt.so.1 /usr/lib/libm.so.6 /usr/lib/libc.so.6 \
    /usr/lib/libstdc++.so.6 /usr/lib/libgcc_s.so.1 \
    /opt/microsoft/azure-mdsd/lib/libtcmalloc_minimal.so.4
find -L /opt/microsoft/azure-mdsd -type f | sort |
while IFS= read -r path; do
    capture azure-mdsd "$path"
done

sort -u "$files" -o "$files"
sort -u "$manifest" -o "$manifest"
# A copied subset can coexist with other files from the base package.
cat /tmp/native-base-manifest "$manifest" | sort -u > "$root/var/lib/rpmmanifest/container-manifest-2"
