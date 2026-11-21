// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/**
 * @title ArtworkMarketplace
 * @notice Fixed contract for 2-way artwork transactions on Ethereum.
 * Creators post artwork sales with cryptographic signatures and designated purchasers.
 * Purchasers buy the artwork with ETH payment and confirm receipt after retrieving
 * the media from the node's Asset Pool.
 */
contract ArtworkMarketplace {
    enum SaleStatus {
        Active,     // Posted for sale
        Purchased,  // Purchased with payment transferred; awaiting media receipt
        Received,   // Purchaser acknowledged receipt of media asset
        Cancelled   // Cancelled by creator before purchase
    }

    struct Sale {
        bytes32 assetId;             // Keccak256 hash of media file
        address payable creator;     // Creator of the artwork
        uint256 price;               // Price in wei
        address designatedPurchaser; // address(0) for open sale, or designated address
        address purchaser;           // Actual purchaser
        SaleStatus status;           // Current sale status
        uint256 postedAt;            // Timestamp of posting
        uint256 purchasedAt;         // Timestamp of purchase
        uint256 receivedAt;          // Timestamp of receipt confirmation
        bytes creatorSignature;      // Creator's cryptographic signature
        string title;                // Artwork title
    }

    // Mapping from assetId to Sale details
    mapping(bytes32 => Sale) public sales;
    // List of all posted assetIds
    bytes32[] public allAssetIds;

    // Events
    event ArtworkPosted(
        bytes32 indexed assetId,
        address indexed creator,
        uint256 price,
        address indexed designatedPurchaser,
        string title,
        uint256 postedAt
    );

    event ArtworkPurchased(
        bytes32 indexed assetId,
        address indexed creator,
        address indexed purchaser,
        uint256 price,
        uint256 purchasedAt
    );

    event ArtworkReceived(
        bytes32 indexed assetId,
        address indexed purchaser,
        uint256 receivedAt
    );

    event SaleCancelled(
        bytes32 indexed assetId,
        address indexed creator
    );

    error SaleAlreadyExists();
    error SaleNotFound();
    error SaleNotActive();
    error NotCreator();
    error NotPurchaser();
    error UnauthorizedPurchaser();
    error IncorrectPaymentAmount();
    error InvalidAssetId();
    error InvalidSignature();
    error PaymentTransferFailed();

    /**
     * @notice Post an artwork for sale on the blockchain.
     * @param assetId Keccak256 hash of the media content
     * @param price Sale price in wei
     * @param designatedPurchaser Optional designated purchaser (address(0) if public)
     * @param creatorSignature 65-byte secp256k1 signature of creator over assetId
     * @param title Title of the artwork
     */
    function postSale(
        bytes32 assetId,
        uint256 price,
        address designatedPurchaser,
        bytes calldata creatorSignature,
        string calldata title
    ) external {
        if (assetId == bytes32(0)) revert InvalidAssetId();
        if (sales[assetId].assetId != bytes32(0)) revert SaleAlreadyExists();
        if (creatorSignature.length != 65) revert InvalidSignature();

        // Verify creator signature
        address recovered = recoverSigner(assetId, creatorSignature);
        if (recovered != msg.sender) {
            revert InvalidSignature();
        }

        sales[assetId] = Sale({
            assetId: assetId,
            creator: payable(msg.sender),
            price: price,
            designatedPurchaser: designatedPurchaser,
            purchaser: address(0),
            status: SaleStatus.Active,
            postedAt: block.timestamp,
            purchasedAt: 0,
            receivedAt: 0,
            creatorSignature: creatorSignature,
            title: title
        });

        allAssetIds.push(assetId);

        emit ArtworkPosted(assetId, msg.sender, price, designatedPurchaser, title, block.timestamp);
    }

    /**
     * @notice Purchase an artwork in an active sale posting.
     * @param assetId The asset ID to purchase
     */
    function purchaseArtwork(bytes32 assetId) external payable {
        Sale storage sale = sales[assetId];
        if (sale.assetId == bytes32(0)) revert SaleNotFound();
        if (sale.status != SaleStatus.Active) revert SaleNotActive();

        // Check designated purchaser constraint
        if (sale.designatedPurchaser != address(0) && sale.designatedPurchaser != msg.sender) {
            revert UnauthorizedPurchaser();
        }

        if (msg.value != sale.price) revert IncorrectPaymentAmount();

        sale.status = SaleStatus.Purchased;
        sale.purchaser = msg.sender;
        sale.purchasedAt = block.timestamp;

        // Transfer payment to creator
        if (sale.price > 0) {
            (bool success, ) = sale.creator.call{value: msg.value}("");
            if (!success) revert PaymentTransferFailed();
        }

        emit ArtworkPurchased(assetId, sale.creator, msg.sender, sale.price, block.timestamp);
    }

    /**
     * @notice Purchaser confirms receipt of the media asset retrieved from the node.
     * @param assetId The asset ID received
     */
    function confirmReceipt(bytes32 assetId) external {
        Sale storage sale = sales[assetId];
        if (sale.assetId == bytes32(0)) revert SaleNotFound();
        if (sale.status != SaleStatus.Purchased) revert SaleNotActive();
        if (sale.purchaser != msg.sender) revert NotPurchaser();

        sale.status = SaleStatus.Received;
        sale.receivedAt = block.timestamp;

        emit ArtworkReceived(assetId, msg.sender, block.timestamp);
    }

    /**
     * @notice Creator cancels an unsold posting.
     * @param assetId The asset ID to cancel
     */
    function cancelSale(bytes32 assetId) external {
        Sale storage sale = sales[assetId];
        if (sale.assetId == bytes32(0)) revert SaleNotFound();
        if (sale.status != SaleStatus.Active) revert SaleNotActive();
        if (sale.creator != msg.sender) revert NotCreator();

        sale.status = SaleStatus.Cancelled;

        emit SaleCancelled(assetId, msg.sender);
    }

    /**
     * @notice Get all posted asset IDs.
     */
    function getAllAssetIds() external view returns (bytes32[] memory) {
        return allAssetIds;
    }

    /**
     * @notice Get total number of sales posted.
     */
    function totalSales() external view returns (uint256) {
        return allAssetIds.length;
    }

    /**
     * @notice Get detailed sale info for an asset.
     */
    function getSale(bytes32 assetId) external view returns (Sale memory) {
        Sale memory sale = sales[assetId];
        if (sale.assetId == bytes32(0)) revert SaleNotFound();
        return sale;
    }

    /**
     * @notice Check whether a given user can buy the artwork.
     */
    function canPurchase(bytes32 assetId, address buyer) external view returns (bool) {
        Sale memory sale = sales[assetId];
        if (sale.assetId == bytes32(0) || sale.status != SaleStatus.Active) {
            return false;
        }
        if (sale.designatedPurchaser != address(0) && sale.designatedPurchaser != buyer) {
            return false;
        }
        return true;
    }

    /**
     * @dev Recover signer address from signature over assetId (supporting standard EIP-191 & raw).
     */
    function recoverSigner(bytes32 assetId, bytes memory signature) public pure returns (address) {
        bytes32 r;
        bytes32 s;
        uint8 v;

        assembly {
            r := mload(add(signature, 32))
            s := mload(add(signature, 64))
            v := byte(0, mload(add(signature, 96)))
        }

        if (v < 27) {
            v += 27;
        }

        // Try standard EIP-191 personal_sign prefix "\x19Ethereum Signed Message:\n32"
        bytes32 ethSignedMessageHash = keccak256(
            abi.encodePacked("\x19Ethereum Signed Message:\n32", assetId)
        );
        address recovered = ecrecover(ethSignedMessageHash, v, r, s);
        if (recovered != address(0)) {
            return recovered;
        }

        // Fallback to raw hash
        return ecrecover(assetId, v, r, s);
    }
}
