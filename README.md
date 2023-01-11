# gasoline
This a PoC to explore the possibility to embed an arbitrary binary file within a CoreOS ISO distribution.
Two different approaches have been implemented, and they are represented as two different commands to be 
invoked from the command line.

# Approach #1: Add a file to the config.ign 

Usage: gasoline update-ignition-config <iso> <file-to-be-added>

The commands expects at least two mandatory parameters:
1) The ISO to be customized
2) The file to be added

Optionally, a third parameter can be specified to configure the output location of the new ISO. If none is provided, the new ISO will be saved in the same path of the source ISO, with a different name.

## Steps

- Reads the source ISO and unpacks it in a temporary folder
- Extracts the original {{config.ign}} from {{/images/ignition.img}}
- Reads the file to be added, encodes it in base64, and appends it to the current {{storage}} section of the ignition file (which gets save as a new file)
- Recreats a new compressed cpio archive for {{/images/ignition.img}}, with the updated ignition file
- Create a new bootable ISO from the updated content in the temporary folder

# Approach #2: Append a file to ignition.img

Usage: gasoline append-ignition-image <iso> <file-to-be-added>

The commands expects at least two mandatory parameters:
1) The ISO to be customized
2) The file to be added

Optionally, a third parameter can be specified to configure the output location of the new ISO. If none is provided, the new ISO will be saved in the same path of the source ISO, with a different name.

## Steps

- Reads the source ISO and unpacks it in a temporary folder
- Extracts the content of {{/images/ignition.img}} in a temporary folder
- Copy the file to be added in the ignition temp folder
- Recreates a new compressed cpio archive for {{/images/ignition.img}} using the ignition temp folder
- Create a new bootable ISO from the updated content in the temporary folder

