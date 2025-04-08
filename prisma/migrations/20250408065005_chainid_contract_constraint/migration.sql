/*
  Warnings:

  - A unique constraint covering the columns `[chainId,eventName,address]` on the table `Contract` will be added. If there are existing duplicate values, this will fail.

*/
-- DropIndex
DROP INDEX "Contract_eventName_address_idx";

-- DropIndex
DROP INDEX "Contract_eventName_address_key";

-- CreateIndex
CREATE INDEX "Contract_chainId_eventName_address_idx" ON "Contract"("chainId", "eventName", "address");

-- CreateIndex
CREATE UNIQUE INDEX "Contract_chainId_eventName_address_key" ON "Contract"("chainId", "eventName", "address");
