param name string
param location string
param tags object
param subnetId string
param vmSize string
param adminSshPublicKey string
param adminUsername string = 'scenarioadmin'
param customCa bool

var bootstrap = '''
#!/bin/bash
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive
cloud-init status --wait >/dev/null
apt-get update -qq >&2
apt-get install -y --no-install-recommends squid-openssl ca-certificates openssl curl >&2
systemctl stop squid
umask 077
if [ '__CUSTOM_CA__' = 'true' ]; then
  install -d -o root -g proxy -m 0750 /etc/squid/ssl_cert
  if [ ! -s /etc/squid/ssl_cert/ca.key ]; then
    openssl req -x509 -newkey rsa:2048 -sha256 -nodes -days 3650 \
      -subj '/CN=Retained AKS scenario proxy CA' \
      -addext 'basicConstraints=critical,CA:TRUE' \
      -addext 'keyUsage=critical,keyCertSign,cRLSign' \
      -addext 'subjectAltName=DNS:scenario-proxy,IP:10.226.0.4' \
      -keyout /etc/squid/ssl_cert/ca.key -out /etc/squid/ssl_cert/ca.crt >&2
  fi
  openssl x509 -in /etc/squid/ssl_cert/ca.crt -checkend 3600 -noout >&2
  chown root:proxy /etc/squid/ssl_cert/ca.key /etc/squid/ssl_cert/ca.crt
  chmod 0640 /etc/squid/ssl_cert/ca.key
  chmod 0644 /etc/squid/ssl_cert/ca.crt
  if [ ! -d /var/lib/squid/ssl_db ]; then
    install -d -o proxy -g proxy -m 0750 /var/lib/squid
    runuser -u proxy -- /usr/lib/squid/security_file_certgen -c -s /var/lib/squid/ssl_db -M 4MB >&2
  fi
  cat > /etc/squid/squid.conf <<'SQUID_TLS'
http_port 3128 ssl-bump cert=/etc/squid/ssl_cert/ca.crt key=/etc/squid/ssl_cert/ca.key generate-host-certificates=on dynamic_cert_mem_cache_size=4MB
sslcrtd_program /usr/lib/squid/security_file_certgen -s /var/lib/squid/ssl_db -M 4MB
acl step1 at_step SslBump1
ssl_bump peek step1
ssl_bump bump all
tls_outgoing_options cafile=/etc/ssl/certs/ca-certificates.crt
SQUID_TLS
else
  printf 'http_port 3128\n' > /etc/squid/squid.conf
fi
cat >> /etc/squid/squid.conf <<'SQUID_COMMON'
acl nodes src 10.224.0.0/16
acl localhost src 127.0.0.1/32 ::1
acl SSL_ports port 443
acl Safe_ports port 80 443
acl CONNECT method CONNECT
http_access deny !Safe_ports
http_access deny CONNECT !SSL_ports
http_access allow localhost
http_access allow nodes
http_access deny all
cache deny all
logformat scenario %ts.%03tu %>a %rm %>rd %>Hs %Sh/%<a
access_log stdio:/var/log/squid/access.log scenario
visible_hostname scenario-proxy
SQUID_COMMON
chmod 0644 /etc/squid/squid.conf
squid -k parse >&2
systemctl enable squid >&2
systemctl restart squid
systemctl is-active --quiet squid
if [ '__CUSTOM_CA__' = 'true' ]; then
  curl --noproxy '' --proxy http://127.0.0.1:3128 --cacert /etc/squid/ssl_cert/ca.crt \
    --retry 5 --retry-connrefused --retry-delay 5 --fail --silent --show-error \
    --output /dev/null '__PROBE_URL__'
  base64 -w0 /etc/squid/ssl_cert/ca.crt
else
  curl --noproxy '' --proxy http://127.0.0.1:3128 \
    --retry 5 --retry-connrefused --retry-delay 5 --fail --silent --show-error \
    --output /dev/null '__PROBE_URL__'
  printf 'READY'
fi
'''

resource nic 'Microsoft.Network/networkInterfaces@2024-05-01' = {
  name: '${name}-nic'
  location: location
  tags: tags
  properties: {
    ipConfigurations: [{
      name: 'proxy'
      properties: {
        privateIPAllocationMethod: 'Static'
        privateIPAddress: '10.226.0.4'
        subnet: { id: subnetId }
      }
    }]
  }
}

resource vm 'Microsoft.Compute/virtualMachines@2024-07-01' = {
  name: name
  location: location
  tags: tags
  properties: {
    hardwareProfile: { vmSize: vmSize }
    osProfile: {
      computerName: name
      adminUsername: adminUsername
      linuxConfiguration: {
        disablePasswordAuthentication: true
        provisionVMAgent: true
        ssh: { publicKeys: [{ path: '/home/${adminUsername}/.ssh/authorized_keys', keyData: adminSshPublicKey }] }
      }
    }
    storageProfile: {
      imageReference: {
        publisher: 'Canonical'
        offer: 'ubuntu-24_04-lts'
        sku: 'server'
        version: 'latest'
      }
      osDisk: {
        name: '${name}-os'
        createOption: 'FromImage'
        deleteOption: 'Delete'
        managedDisk: { storageAccountType: 'StandardSSD_LRS' }
        diskSizeGB: 32
      }
    }
    networkProfile: { networkInterfaces: [{ id: nic.id, properties: { deleteOption: 'Delete' } }] }
  }
}

resource configure 'Microsoft.Compute/virtualMachines/runCommands@2024-07-01' = {
  parent: vm
  name: 'proxy-bootstrap'
  location: location
  tags: tags
  properties: {
    asyncExecution: false
    treatFailureAsDeploymentFailure: true
    timeoutInSeconds: 1800
    source: {
      script: replace(replace(bootstrap, '__CUSTOM_CA__', toLower(string(customCa))), '__PROBE_URL__', '${environment().resourceManager}metadata/endpoints?api-version=2020-06-01')
    }
  }
}

output vmId string = vm.id
output vmName string = vm.name
output endpoint string = 'http://10.226.0.4:3128'
output bootstrapCommandId string = configure.id
