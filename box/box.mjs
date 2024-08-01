import 'dotenv/config'
import assert from 'node:assert'
import BoxSDK from 'box-node-sdk'

export default class Box {
    constructor() {
        assert(process.env.BOX_CLIENT_ID !== undefined, "Client ID is equal to undefined, meaning that there's something wrong with how the environmental variables are working.")
        assert(process.env.BOX_CLIENT_SECRET !== undefined, "Client Secret is equal to undefined, meaning that there's something wrong with how the environmental variables are working.")
        assert(process.env.BOX_PUBLIC_KEY_ID !== undefined, "Public Key ID is equal to undefined, meaning that there's something wrong with how the environmental variables are working.")
        assert(process.env.BOX_PRIVATE_KEY !== undefined, "Private Key is equal to undefined, meaning that there's something wrong with how the environmental variables are working.")
        assert(process.env.BOX_PASSPHRASE !== undefined, "Passphrase is equal to undefined, meaning that there's something wrong with how the environmental variables are working.")
        assert(process.env.BOX_USER_ID !== undefined, "User ID is equal to undefined, meaning that there's something wrong with how the environmental variables are working.")

        this.userID = process.env.BOX_USER_ID
        this.enterpriseID = process.env.BOX_ENTERPRISE_ID
        this.folderID = process.env.BOX_FOLDER_ID

        this.config = {
            clientID: process.env.BOX_CLIENT_ID,
            clientSecret: process.env.BOX_CLIENT_SECRET,
            appAuth: {
                keyID: process.env.BOX_PUBLIC_KEY_ID,
                privateKey: process.env.BOX_PRIVATE_KEY,
                passphrase: process.env.BOX_PASSPHRASE
            }
        }
        this.sdk = new BoxSDK(this.config)
        this.client = this.sdk.getAppAuthClient('enterprise', this.enterpriseID)

    }
    async listItemsInFolder(folderID) {
        /**
         * ```js
         * const TypeItemsStruct = {
         *  total_count: 0,
                entries: [
                    {
                        type: 'file',
                        id: "12345",
                        etag: "0",
                        name: ""
        
                    }
                ],
                offset: 0,
                limit: 25,
                order: [
                    { by: 'type', direction: 'ASC' },
                    { by: 'name', direction: 'ASC' }
                ]
        * }
        * ```
        */
        assert(folderID !== undefined, "folderID is undefined, meaning that it wasn't passed into this function")
        assert(typeof folderID === 'string', "Typeof folderID is not string, actual type is ", typeof folderID)
        this.files = await this.client.folders.getItems(folderID, {
            usemarker: 'false',
            fields: 'name',
            offset: 0,
            limit: 25
        })
        return this.files
    }
    async getFile(fileID) {
        assert(fileID !== undefined, "fileID is undefined, meaning that it wasn't passed into this function")
        assert(typeof fileID === 'string', "Typeof fileID is not string, actual type is ", typeof fileID)
        const fileURL = await this.client.files.getDownloadURL(fileID)
        return fileURL
    }
    async getRandomFile(randomNumber) {
        const response = {
            fileID: "",
            fileName: "",
            url: "",
        }
        assert(randomNumber !== undefined, "Random number is undefined, meaning that it wasn't properly passed to the function.")
        assert(typeof randomNumber === "number", "typeof randomNumber is not number, and this function requires it to be a number.")
        assert(this.files !== undefined, "File are undefined - make sure to call listItemsInFolder() first and that there are files in the requested folder.")
        assert(this.files.entries.length > randomNumber, `Random number exceeds range of available files. Random number is: ${randomNumber}. Entries are: ${this.files.entries.length}`)


        response.fileID = this.files?.entries[randomNumber]?.id
        response.fileName = this.files?.entries[randomNumber]?.name
        response.url = await this.getFile(response.fileID)
        return response
    }
    async uploadFile(filename, stream, options) {
        const request = await this.client.files.uploadFile(this.folderID, filename, stream, options)
        return request
    }
}