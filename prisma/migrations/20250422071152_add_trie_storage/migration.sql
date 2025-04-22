-- CreateTable
CREATE TABLE "StorageKV" (
    "id" TEXT NOT NULL,
    "key" TEXT NOT NULL,
    "value" BYTEA NOT NULL,
    "createdAt" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updatedAt" TIMESTAMP(3) NOT NULL,

    CONSTRAINT "StorageKV_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "CodeStorage" (
    "id" TEXT NOT NULL,
    "hash" TEXT NOT NULL,
    "code" BYTEA NOT NULL,
    "createdAt" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updatedAt" TIMESTAMP(3) NOT NULL,

    CONSTRAINT "CodeStorage_pkey" PRIMARY KEY ("id")
);

-- CreateIndex
CREATE UNIQUE INDEX "StorageKV_key_key" ON "StorageKV"("key");

-- CreateIndex
CREATE INDEX "StorageKV_key_idx" ON "StorageKV"("key");

-- CreateIndex
CREATE UNIQUE INDEX "CodeStorage_hash_key" ON "CodeStorage"("hash");

-- CreateIndex
CREATE INDEX "CodeStorage_hash_idx" ON "CodeStorage"("hash");
