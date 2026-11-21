## Requirements

1. Two users, a creator of an artwork and a purchaser should be able to perform a 2-way transaction on the blockchain without the use of external platforms like ipfs.

2. First part of the transaction is the payment part which is already handled well in ethereum.

3. Second part of the transaction is sending the image/asset to the purchaser. This involves ensuring that the file being sent has the correct signatures. It is assumed that the original artwork is the one that was "advertised" on the blockchain by whatever means. So the case where the creator of the artwork advertises a fake/clone of the artwork, verifying the legitimacy of the artwork is the purchasers perogative.

4. The media should be persisted on the blockchain at least for a reasonable enough time where after said time, it can be assumed that the purchaser definitely received the file. This will ensure that the cost of persisting the file is reasonable.

## Proposed Architecture

### Asset Pool

Similar to a mempool, but contains instead the assets which are currently assumed to not be received completely by the purchaser.

### Purchaser time window

This will be 48 hours (for now). The purchaser should be able to receive the artwork within this time frame, after which the nodes get rid of the file from the asset pool.

### Sale posting transaction

This kind of transaction is made to some fixed address owned by the blockchain. Any user can look up the these posting transactions and send a request to purchase artwork in said posting.

This type of transaction also allows the user to specify an address as the purchaser, in which case only that address will be able to buy the artwork.
