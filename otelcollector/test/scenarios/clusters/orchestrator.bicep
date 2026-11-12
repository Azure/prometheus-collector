param name string
param location string
param tags object
param subnetId string
param adminSshPublicKey string
param vmSize string = 'Standard_D2s_v4'

resource nsg 'Microsoft.Network/networkSecurityGroups@2024-05-01' = {
  name: '${name}-nsg'
  location: location
  tags: tags
  properties: {
    securityRules: [
    {
      name: 'ActivePipelineTunnel'
      properties: {
        priority: 100
        access: 'Deny'
        direction: 'Inbound'
        protocol: 'Tcp'
        sourceAddressPrefix: '*'
        sourcePortRange: '*'
        destinationAddressPrefix: '*'
        destinationPortRange: '22'
      }
    }
    {
      name: 'DenyInbound'
      properties: {
        priority: 200
        access: 'Deny'
        direction: 'Inbound'
        protocol: '*'
        sourceAddressPrefix: '*'
        sourcePortRange: '*'
        destinationAddressPrefix: '*'
        destinationPortRange: '*'
      }
    }]
  }
}

resource ip 'Microsoft.Network/publicIPAddresses@2024-05-01' = {
  name: '${name}-ip'
  location: location
  tags: tags
  sku: { name: 'Standard' }
  properties: { publicIPAllocationMethod: 'Static' }
}

resource nic 'Microsoft.Network/networkInterfaces@2024-05-01' = {
  name: '${name}-nic'
  location: location
  tags: tags
  properties: {
    networkSecurityGroup: { id: nsg.id }
    ipConfigurations: [{
      name: 'orchestrator'
      properties: {
        privateIPAllocationMethod: 'Dynamic'
        subnet: { id: subnetId }
        publicIPAddress: { id: ip.id }
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
      adminUsername: 'scenarioadmin'
      linuxConfiguration: {
        disablePasswordAuthentication: true
        provisionVMAgent: true
        ssh: { publicKeys: [{
          path: '/home/scenarioadmin/.ssh/authorized_keys'
          keyData: adminSshPublicKey
        }] }
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
        createOption: 'FromImage'
        deleteOption: 'Delete'
        managedDisk: { storageAccountType: 'StandardSSD_LRS' }
        diskSizeGB: 32
      }
    }
    networkProfile: { networkInterfaces: [{ id: nic.id, properties: { deleteOption: 'Delete' } }] }
  }
}

output vmName string = vm.name
output nsgName string = nsg.name
output publicIp string = ip.properties.ipAddress
output vmId string = vm.id
