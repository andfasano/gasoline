# gasoline
This a PoC to explore the possibility to embed an arbitrary binary file within a CoreOS ISO distribution.

The util expects at least two mandatory parameters:
1) The ISO to be customized
2) The file to be added

Optionally, a third parameter can be specified to configure the output location of the new ISO. If none is provided, the new ISO will be saved in the same path of the source ISO, with a different name.

## Steps

The PoC performs the following steps:

- Reads the source ISO and unpacks it in a temporary folder
- Extracts the original {{config.ign}} from {{/images/ignition.img}}
- Reads the file to be added, encodes it in base64, and appends it to the current {{storage}} section of the ignition file (which gets save as a new file)
- Recreats a new compressed cpio archive for {{/images/ignition.img}}, with the updated ignition file
- Create a new bootable ISO from the updated content in the temporary folder
