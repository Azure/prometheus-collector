#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/../.."
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
for dockerfile in otelcollector/build/linux/Dockerfile otelcollector/build/linux/ccp/Dockerfile; do
    tr -d '\r' < "$dockerfile" > "$tmp/dockerfile"
    grep -qx 'FROM builder AS native-capture' "$tmp/dockerfile"
    grep -qx 'COPY --from=runtime-base /var/lib/rpmmanifest/container-manifest-2 /tmp/native-base-manifest' "$tmp/dockerfile"
    grep -qx 'RUN bash /opt/capture-native-packages.sh' "$tmp/dockerfile"
    grep -qx 'COPY --from=native-capture /opt /opt' "$tmp/dockerfile"
    test "$(grep '^COPY ' "$tmp/dockerfile" | tail -n 1)" = 'COPY --from=native-capture /native-rootfs/ /'
done
mkdir -p "$tmp/bin" "$tmp/usr/sbin" "$tmp/usr/lib" "$tmp/opt/microsoft/azure-mdsd/lib"

cat > "$tmp/bin/rpm" <<'EOF'
#!/usr/bin/env bash
if [[ $1 == -qf ]]; then
    case "$4" in
        */MetricsExtension) echo metricsext2 ;;
        */mdsd|*/microsoft/azure-mdsd/*) echo azure-mdsd ;;
        *) echo shared ;;
    esac
elif [[ $4 == *INSTALLTIME* ]]; then
    printf '%s\t1-2\t1\t1\tVendor\t(none)\t1\tx86_64\t0\tsource.rpm\n' "$2"
else
    printf '0:1-2'
fi
EOF
chmod +x "$tmp/bin/rpm"
export PATH="$tmp/bin:$PATH"
for path in MetricsExtension mdsd; do echo "$path" > "$tmp/usr/sbin/$path"; done
for lib in libcpprest.so.1 libuuid.so.1 libstdc++.so.6 libm.so.6 libgcc_s.so.1 libc.so.6 \
    libboost_atomic.so.1 libbrotlidec.so.1 libbrotlienc.so.1 libz.so.1 libbrotlicommon.so.1 \
    libpthread.so.0 libdl.so.2 libsymcrypt.so.103 librt.so.1; do
    echo "$lib" > "$tmp/usr/lib/$lib"
done
echo boost > "$tmp/usr/lib/libboost_filesystem.so.1.0"
ln -s libboost_filesystem.so.1.0 "$tmp/usr/lib/libboost_filesystem.so.1"
echo mdsd-lib > "$tmp/opt/microsoft/azure-mdsd/lib/libtcmalloc_minimal.so.4"
printf 'shared\t0-0\t1\t1\tVendor\t(none)\t1\tx86_64\t0\tsource.rpm\nuntouched\tbase\n' > "$tmp/base"
sed -e "s|/native-rootfs|$tmp/rootfs|g" \
    -e "s|/tmp/native-base-manifest|$tmp/base|g" \
    -e "s|/opt/|$tmp/opt/|g" -e "s|/usr/|$tmp/usr/|g" \
    otelcollector/scripts/capture-native-packages.sh > "$tmp/capture.sh"
bash "$tmp/capture.sh"
manifest="$tmp/rootfs/var/lib/rpmmanifest/container-manifest-2"
grep -q $'^untouched\tbase$' "$manifest"
grep -q $'^shared\t0-0\t' "$manifest"
for name in metricsext2 azure-mdsd shared; do
    grep -q "^${name}"$'\t1-2\t' "$manifest"
done
test ! -L "$tmp/rootfs$tmp/usr/lib/libboost_filesystem.so.1"
while IFS=$'\t' read -r consumer owner version path hash; do
    if [[ $path == "$tmp/opt/"* ]]; then actual=$path; else actual="$tmp/rootfs$path"; fi
    test "$(sha256sum "$actual" | cut -d' ' -f1)" = "$hash"
done < "$tmp/opt/native-packages/files.tsv"
rm "$tmp/usr/sbin/MetricsExtension"
if bash "$tmp/capture.sh" >/dev/null 2>&1; then echo 'Missing native file accepted' >&2; exit 1; fi
echo 'Native RPM capture checks passed'
