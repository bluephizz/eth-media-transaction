// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import {Test, console} from "forge-std/Test.sol";
import {ArtworkMarketplace} from "../src/ArtworkMarketplace.sol";

contract ArtworkMarketplaceTest is Test {
    ArtworkMarketplace public marketplace;

    uint256 internal creatorPrivateKey;
    address internal creator;

    uint256 internal purchaserPrivateKey;
    address internal purchaser;

    uint256 internal otherUserPrivateKey;
    address internal otherUser;

    function setUp() public {
        marketplace = new ArtworkMarketplace();

        creatorPrivateKey = 0xA11CE;
        creator = vm.addr(creatorPrivateKey);
        vm.deal(creator, 100 ether);

        purchaserPrivateKey = 0xB0B;
        purchaser = vm.addr(purchaserPrivateKey);
        vm.deal(purchaser, 100 ether);

        otherUserPrivateKey = 0xCAFE;
        otherUser = vm.addr(otherUserPrivateKey);
        vm.deal(otherUser, 100 ether);
    }

    function testPostSaleAndPurchase() public {
        bytes32 assetId = keccak256("ARTWORK_MEDIA_BYTES_123");
        bytes32 ethSignedMsgHash = keccak256(
            abi.encodePacked("\x19Ethereum Signed Message:\n32", assetId)
        );
        (uint8 v, bytes32 r, bytes32 s) = vm.sign(creatorPrivateKey, ethSignedMsgHash);
        bytes memory signature = abi.encodePacked(r, s, v);

        uint256 price = 2 ether;

        // Post sale
        vm.prank(creator);
        marketplace.postSale(assetId, price, address(0), signature, "Sunset Over Mountain");

        assertEq(marketplace.totalSales(), 1);

        ArtworkMarketplace.Sale memory sale = marketplace.getSale(assetId);
        assertEq(sale.creator, creator);
        assertEq(sale.price, price);
        assertEq(uint8(sale.status), uint8(ArtworkMarketplace.SaleStatus.Active));

        // Purchaser purchases artwork
        uint256 creatorBalanceBefore = creator.balance;
        vm.prank(purchaser);
        marketplace.purchaseArtwork{value: price}(assetId);

        sale = marketplace.getSale(assetId);
        assertEq(uint8(sale.status), uint8(ArtworkMarketplace.SaleStatus.Purchased));
        assertEq(sale.purchaser, purchaser);
        assertEq(creator.balance, creatorBalanceBefore + price);

        // Purchaser confirms receipt
        vm.prank(purchaser);
        marketplace.confirmReceipt(assetId);

        sale = marketplace.getSale(assetId);
        assertEq(uint8(sale.status), uint8(ArtworkMarketplace.SaleStatus.Received));
    }

    function testDesignatedPurchaserRestriction() public {
        bytes32 assetId = keccak256("EXCLUSIVE_VIP_ARTWORK");
        bytes32 ethSignedMsgHash = keccak256(
            abi.encodePacked("\x19Ethereum Signed Message:\n32", assetId)
        );
        (uint8 v, bytes32 r, bytes32 s) = vm.sign(creatorPrivateKey, ethSignedMsgHash);
        bytes memory signature = abi.encodePacked(r, s, v);

        uint256 price = 1 ether;

        // Post sale designated only for `purchaser`
        vm.prank(creator);
        marketplace.postSale(assetId, price, purchaser, signature, "VIP Art");

        // Other user attempts to buy and must fail
        vm.prank(otherUser);
        vm.expectRevert(ArtworkMarketplace.UnauthorizedPurchaser.selector);
        marketplace.purchaseArtwork{value: price}(assetId);

        // Designated purchaser buys successfully
        vm.prank(purchaser);
        marketplace.purchaseArtwork{value: price}(assetId);

        ArtworkMarketplace.Sale memory sale = marketplace.getSale(assetId);
        assertEq(sale.purchaser, purchaser);
    }

    function testInvalidSignatureRejected() public {
        bytes32 assetId = keccak256("ARTWORK_FAKE_SIG");
        bytes32 ethSignedMsgHash = keccak256(
            abi.encodePacked("\x19Ethereum Signed Message:\n32", assetId)
        );
        // Signed with OTHER user key
        (uint8 v, bytes32 r, bytes32 s) = vm.sign(otherUserPrivateKey, ethSignedMsgHash);
        bytes memory signature = abi.encodePacked(r, s, v);

        // Creator posts with spoofed signature -> must revert
        vm.prank(creator);
        vm.expectRevert(ArtworkMarketplace.InvalidSignature.selector);
        marketplace.postSale(assetId, 1 ether, address(0), signature, "Spoofed Title");
    }

    function testCancelSale() public {
        bytes32 assetId = keccak256("ARTWORK_CANCEL_ME");
        bytes32 ethSignedMsgHash = keccak256(
            abi.encodePacked("\x19Ethereum Signed Message:\n32", assetId)
        );
        (uint8 v, bytes32 r, bytes32 s) = vm.sign(creatorPrivateKey, ethSignedMsgHash);
        bytes memory signature = abi.encodePacked(r, s, v);

        vm.prank(creator);
        marketplace.postSale(assetId, 1 ether, address(0), signature, "To Cancel");

        vm.prank(creator);
        marketplace.cancelSale(assetId);

        ArtworkMarketplace.Sale memory sale = marketplace.getSale(assetId);
        assertEq(uint8(sale.status), uint8(ArtworkMarketplace.SaleStatus.Cancelled));
    }
}
