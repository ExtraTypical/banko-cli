#!/usr/bin/env node
import Box from "../box/box.mjs";
import asciify from "asciify-image";
import 'dotenv/config';




async function run() {
    const box = new Box()
    const items = await box.listItemsInFolder(process.env.BOX_FOLDER_ID)
    /** Error handling for items response */
    switch (true) {
        case (items === undefined): {
            const error = "items returned as undefined, which means there was a problem with the call"
            console.error(error)
            return error
        }
        case (items.total_count === undefined): {
            const error = "items.total_count is equal to undefined, meaning that there's something wrong with the payload"
            console.error(error)
            return error
        }
        case (items?.total_count === 0): {
            const error = "Total count of files is equal to 0, which should not be the case."
            console.error(error)
            return error
        }
        case (items?.entries === undefined): {
            const error = "Entries are undefined, meaning there's a problem with the payload"
            console.error(error)
            return error
        }
    }

    const min = 0
    const max = items?.entries.length
    const randomNumber = Math.floor(Math.random() * (max - min)) + min
    const file = await box.getRandomFile(randomNumber)

    const options = {
        fit: 'box',
        width: 80,
        height: 100
    }

    asciify(file.url, options, function (err, asciified) {
        if (err) throw err;

        // Print to console
        console.log(asciified);
    });


}
run()