package cli

const homeDepotObservedAddQuery = `mutation addToCart($cartRequest: CartInfoRequest!, $requestContext: RequestContext) {
  addToCart(cartRequest: $cartRequest, requestContext: $requestContext) {
    cartId
    itemCount
    customer {
      userId
      customerId
      type
      addresses {
        id
        firstName
        lastName
        addressLine1
        addressLine2
        zipCode
        state
        country
        county
        phone
        phoneNumber
        hideCVV
        city
        type
        default
        primaryPhoneId
        addressIdentifier
        businessAddress
        category
        __typename
      }
      __typename
    }
    payments {
      emailId
      amountCharged
      cardBrand
      maskedCardNumber
      xref
      paymentId
      type
      addressIds
      __typename
    }
    rentalEstimate {
      tools {
        categoryCode
        subCategoryCode
        deposit
        feeTotal
        taxTotal
        taxPercentage
        fees {
          type
          value
          taxTotal
          taxPercentage
          percentage
          taxes {
            type
            value
            percentage
            __typename
          }
          __typename
        }
        __typename
      }
      __typename
    }
    messages {
      type
      messageCategoryType
      correlationId
      correlationType
      longDesc
      shortDesc
      __typename
    }
    items {
      id
      quantity
      product {
        itemId
        addons {
          installation
          termLength
          type
          price
          totalPrice
          quantity
          itemId
          id
          category
          description
          detailsUrl
          selected
          storeId
          protectionPlanParentId
          brandName
          configAttr
          descriptiveAttributes
          media {
            images {
              url
              type
              subType
              sizes
              hotspots
              altText
              __typename
            }
            __typename
          }
          identifiers {
            configId
            editUrl
            copyUrl
            productCategory
            leadTime
            canonicalUrl
            brandName
            itemId
            modelNumber
            productLabel
            storeSkuNumber
            skuClassification
            productType
            isSuperSku
            shipType
            partNumber
            fromName
            toName
            message
            deliveryMethod
            recipientEmail
            __typename
          }
          __typename
        }
        info {
          returnable
          quantityLimit
          minimumOrderQuantity
          inStoreReturnEligibility
          paintBrand
          paintDetails {
            configId
            colorName
            rgb {
              red
              blue
              green
              __typename
            }
            __typename
          }
          __typename
        }
        specificationGroup {
          specTitle
          specifications {
            specName
            specValue
            __typename
          }
          __typename
        }
        pricing {
          value
          original
          total
          totalWithNoDiscount
          valueStartDate
          valueEndDate
          type
          discount {
            percentOff
            dollarOff
            __typename
          }
          clearance {
            value
            dollarOff
            percentageOff
            __typename
          }
          mapDetail {
            mapPolicy
            __typename
          }
          __typename
        }
        media {
          images {
            url
            type
            subType
            sizes
            hotspots
            altText
            __typename
          }
          __typename
        }
        identifiers {
          configId
          editUrl
          copyUrl
          productCategory
          leadTime
          canonicalUrl
          brandName
          itemId
          modelNumber
          productLabel
          storeSkuNumber
          skuClassification
          productType
          isSuperSku
          shipType
          partNumber
          fromName
          toName
          message
          deliveryMethod
          recipientEmail
          __typename
        }
        fulfillment {
          backordered
          backorderedShipDate
          bossExcludedShipStates
          excludedShipStates
          seasonStatusEligible
          anchorStoreStatus
          anchorStoreStatusType
          sthExcludedShipState
          bossExcludedShipState
          onlineStoreStatus
          onlineStoreStatusType
          inStoreAssemblyEligible
          fulfillmentOptions {
            type
            fulfillable
            services {
              type
              expectedArrival
              hasFreeShipping
              estimatedDelivery
              freeDeliveryThreshold
              deliveryCharge
              selected
              optimalFulfillment
              dynamicEta {
                hours
                minutes
                __typename
              }
              deliveryDates {
                startDate
                endDate
                __typename
              }
              totalCharge
              deliveryTimeline
              locations {
                isAnchor
                locationId
                zipCode
                curbsidePickupFlag
                isBuyInStoreCheckNearBy
                distance
                storeName
                city
                state
                storePhone
                type
                inventory {
                  isOutOfStock
                  quantity
                  isInStock
                  isUnavailable
                  isLimitedQuantity
                  backordered
                  maxAllowedBopisQty
                  minAllowedBopisQty
                  __typename
                }
                storeHours {
                  monday {
                    open
                    close
                    __typename
                  }
                  tuesday {
                    open
                    close
                    __typename
                  }
                  wednesday {
                    open
                    close
                    __typename
                  }
                  thursday {
                    open
                    close
                    __typename
                  }
                  friday {
                    open
                    close
                    __typename
                  }
                  saturday {
                    open
                    close
                    __typename
                  }
                  sunday {
                    open
                    close
                    __typename
                  }
                  storeTimeZone
                  __typename
                }
                __typename
              }
              isBossDominant
              hasSameDayCarDelivery
              sameDayDeliveryCharge
              vehicleType
              __typename
            }
            addressId
            priorityDeliveryOptions {
              timeline
              type
              totalCharge
              date
              __typename
            }
            programInclusions {
              description
              threshold
              __typename
            }
            __typename
          }
          __typename
        }
        dataSources
        attributes {
          name
          value
          sequenceNumber
          __typename
        }
        essentialAccessories
        bundleSpecification {
          brand
          experienceType
          type
          __typename
        }
        __typename
      }
      selectedFulfillment
      __typename
    }
    itemGrouping {
      byFulfillment {
        appliance {
          type
          zipCode
          ids
          __typename
        }
        pickup {
          location {
            curbsidePickupFlag
            storeName
            __typename
          }
          ids
          __typename
        }
        delivery {
          grouping {
            type
            ids
            __typename
          }
          __typename
        }
        __typename
      }
      __typename
    }
    descriptiveAttr {
      cartType
      paypalExpress
      paymentOnHold
      isIcEnabled
      displayIcOption
      poJobName
      projectId
      hasSubscriptionItems
      maxCartPriceContributor
      uniqueItemCount
      __typename
    }
    promos {
      desc
      longDesc
      type
      tag
      appliedDisc
      promoCode
      restrictions {
        paymentType
        __typename
      }
      message {
        type
        messageCategoryType
        correlationId
        correlationType
        longDesc
        shortDesc
        __typename
      }
      attached
      promoItems {
        appliedDiscount
        appliedOn
        __typename
      }
      attributes {
        name
        value
        __typename
      }
      __typename
    }
    localization {
      primaryStoreId
      deliveryZip
      deliveryStateCode
      __typename
    }
    totals {
      total
      totalDiscount
      totalWithNoDiscount
      deliveryCharge
      applianceDeliveryCharge
      type
      adjustments {
        amount
        type
        __typename
      }
      plccEligibleTotal
      __typename
    }
    __typename
  }
}`
